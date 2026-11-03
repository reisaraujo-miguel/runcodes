package services

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/storage"
)

const (
	// MaxSubmissionBytes is the maximum accepted source file size (10 MiB).
	MaxSubmissionBytes = 10 << 20

	// SubmissionHistoryLimit bounds how many past submissions are returned for
	// one exercise, keeping the response and the history dialog small.
	SubmissionHistoryLimit = 50

	// MaxPendingSubmissionsPerUser bounds a single account's unfinished runs.
	// The judge concurrency is shared by the whole platform, so one student must
	// not be able to occupy every worker for themselves.
	MaxPendingSubmissionsPerUser = 2

	// wakeTimeout bounds the judge wake-up call so a submission request cannot
	// hang on an unresponsive judge.
	wakeTimeout = 5 * time.Second
)

/*
SubmissionInput carries everything needed to register a submission.
*/
type SubmissionInput struct {
	UserID      int64
	ExerciseID  int64
	IP          *string
	Filename    string
	File        io.Reader
	Size        int64
	ContentType string
}

/*
CreateSubmission validates the submission, checks the judge is healthy, uploads
the source to S3, inserts the commit row and then wakes the judge.

The row is committed before the wake on purpose. The judge claims from Postgres,
so it cannot see an uncommitted row: waking inside the transaction (as this used
to) made every wake a no-op and pushed each submission to the judge's next poll
tick, which is where the queue time in the statistics came from.

The wake is an optimization over that poll loop, so a failed wake does not fail
the submission: the row stays `queued` and is claimed as soon as the judge is
reachable again. Only a failure *before* the row exists (the upload, the insert)
cleans up the uploaded object.
*/
func CreateSubmission(ctx context.Context, in SubmissionInput) (int64, error) {
	if err := validateExercise(ctx, in.ExerciseID, in.UserID); err != nil {
		return 0, err
	}

	if err := validateTestCases(ctx, in.ExerciseID); err != nil {
		return 0, err
	}

	fileTypeID, fileTypeMatched, err := resolveFileType(
		ctx, in.ExerciseID, in.Filename,
	)
	if err != nil {
		return 0, err
	}

	// Keep the catalog entry nullable: an exercise without allowed types accepts
	// any file, and the commit then has no language to attribute. database/sql
	// writes the untyped nil as SQL NULL.
	var fileType any
	if fileTypeMatched {
		fileType = fileTypeID
	}

	// Refuse work before uploading anything when the caller already has runs in
	// flight or the shared queue is full. This is admission control, not a
	// security boundary: the checks are deliberately cheap and fail fast.
	if err := checkSubmissionAdmission(ctx, in.UserID); err != nil {
		return 0, err
	}

	// Do not register anything if the judge cannot run the submission.
	if err := judge.Ready(ctx); err != nil {
		slog.ErrorContext(ctx, "judge is not ready",
			slog.String("error", err.Error()),
		)
		return 0, ErrJudgeUnavailable
	}

	key := buildCommitKey(in.Filename)

	if err := storage.UploadCommitSource(
		ctx, key, in.File, in.Size, in.ContentType,
	); err != nil {
		slog.ErrorContext(ctx, "failed to upload submission source",
			slog.Int64("user_id", in.UserID),
			slog.Int64("exercise_id", in.ExerciseID),
			slog.String("error", err.Error()),
		)
		return 0, ErrServer
	}

	// Register the commit. It becomes visible to the judge's claim only here, so
	// the wake below must happen after this commit, never inside the
	// transaction.
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to begin submission transaction",
			slog.String("error", err.Error()),
		)
		deleteSourceBestEffort(ctx, key)
		return 0, ErrServer
	}
	defer tx.Rollback()

	var commitID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO commits
		     (user_id, exercise_id, status, ip, s3_key, allowed_file_type_id)
		 VALUES ($1, $2, 'queued', $3, $4, $5)
		 RETURNING id`,
		in.UserID, in.ExerciseID, in.IP, key, fileType,
	).Scan(&commitID); err != nil {
		slog.ErrorContext(ctx, "failed to insert commit",
			slog.Int64("user_id", in.UserID),
			slog.Int64("exercise_id", in.ExerciseID),
			slog.String("error", err.Error()),
		)
		deleteSourceBestEffort(ctx, key)
		return 0, ErrServer
	}

	// Freeze the limits this submission runs under: the judge grades against this
	// snapshot, so editing the exercise afterwards cannot change a queued run.
	// The helper reads the exercise and language back from the row it just wrote,
	// which is what resolves a ghost exercise to the one that owns the cases.
	if err := SnapshotCommitLimits(ctx, tx, []int64{commitID}); err != nil {
		deleteSourceBestEffort(ctx, key)
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "failed to commit submission",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		deleteSourceBestEffort(ctx, key)
		return 0, ErrServer
	}

	// Nudge the judge now that the row is visible. A failure is logged, not
	// returned: the commit is durable and the poll loop will claim it, so the
	// submission is accepted either way. Rejecting it here would delete a row the
	// judge may already have claimed.
	wakeCtx, cancel := context.WithTimeout(ctx, wakeTimeout)
	if err := judge.Wake(wakeCtx, commitID); err != nil {
		slog.WarnContext(ctx, "could not wake the judge; the commit stays queued",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
	}
	cancel()

	// Consume the judge stream immediately, even if nobody is watching yet:
	// results must be persisted for a user who reloads or comes back later.
	judge.Consume(commitID)

	slog.InfoContext(ctx, "submission queued",
		slog.Int64("commit_id", commitID),
		slog.Int64("user_id", in.UserID),
		slog.Int64("exercise_id", in.ExerciseID),
	)

	return commitID, nil
}

/*
deleteSourceBestEffort removes an uploaded object, logging (but not returning)
any failure.
*/
func deleteSourceBestEffort(ctx context.Context, key string) {
	if err := storage.DeleteCommitSource(ctx, key); err != nil {
		slog.ErrorContext(ctx, "failed to clean up uploaded source",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
	}
}

/*
checkSubmissionAdmission limits a single user's unfinished submissions and the
global queue depth. The per-user limit keeps one account from occupying every
judge worker; the queue limit keeps accepted work from growing without bound
while the judge is slow or down. Both are best-effort checks: two concurrent
requests can both observe room, which is acceptable for a classroom workload.
*/
