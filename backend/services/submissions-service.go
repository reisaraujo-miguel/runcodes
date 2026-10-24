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
the source to S3, inserts the commit row and wakes the judge. If the judge
cannot be woken, the row is removed and the uploaded object deleted so no
submission is silently lost.
*/
func CreateSubmission(ctx context.Context, in SubmissionInput) (int64, error) {
	if err := validateExercise(ctx, in.ExerciseID, in.UserID); err != nil {
		return 0, err
	}

	if err := validateTestCases(ctx, in.ExerciseID); err != nil {
		return 0, err
	}

	if err := validateFileType(ctx, in.ExerciseID, in.Filename); err != nil {
		return 0, err
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

	// Register the commit and wake the judge inside one transaction, waking
	// BEFORE committing. The judge's poller must not see the row until the wake
	// has succeeded; otherwise a failed wake could delete a commit the judge had
	// already claimed (orphaning a running container and losing the row).
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
		`INSERT INTO commits (user_id, exercise_id, status, ip, s3_key)
		 VALUES ($1, $2, 'queued', $3, $4)
		 RETURNING id`,
		in.UserID, in.ExerciseID, in.IP, key,
	).Scan(&commitID); err != nil {
		slog.ErrorContext(ctx, "failed to insert commit",
			slog.Int64("user_id", in.UserID),
			slog.Int64("exercise_id", in.ExerciseID),
			slog.String("error", err.Error()),
		)
		deleteSourceBestEffort(ctx, key)
		return 0, ErrServer
	}

	wakeCtx, cancel := context.WithTimeout(ctx, wakeTimeout)
	defer cancel()

	if err := judge.Wake(wakeCtx, commitID); err != nil {
		slog.ErrorContext(ctx, "failed to wake judge for commit",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		// The deferred rollback removes the (never committed) row; drop the
		// uploaded object so no submission is silently left behind.
		deleteSourceBestEffort(ctx, key)
		return 0, ErrJudgeUnavailable
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "failed to commit submission",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		deleteSourceBestEffort(ctx, key)
		return 0, ErrServer
	}

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
