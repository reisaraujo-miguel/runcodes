package services

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"path"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/models"

	"github.com/google/uuid"
)

const (
	// MaxSubmissionBytes is the maximum accepted source file size (10 MiB).
	MaxSubmissionBytes = 10 << 20

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

	if err := validateFileType(ctx, in.ExerciseID, in.Filename); err != nil {
		return 0, err
	}

	// Do not register anything if the judge cannot run the submission.
	if err := JudgeReady(ctx); err != nil {
		slog.ErrorContext(ctx, "judge is not ready",
			slog.String("error", err.Error()),
		)
		return 0, ErrJudgeUnavailable
	}

	key := buildCommitKey(in.Filename)

	if err := UploadCommitSource(
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
	tx, err := DB.BeginTx(ctx, nil)
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

	if err := WakeJudge(wakeCtx, commitID); err != nil {
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
	getOrCreateHub(commitID)

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
	if err := DeleteCommitSource(ctx, key); err != nil {
		slog.ErrorContext(ctx, "failed to clean up uploaded source",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
	}
}

/*
validateExercise checks the exercise exists, is not removed and belongs to an
offering the user is enrolled in, and that its deadline has not passed.
*/
func validateExercise(ctx context.Context, exerciseID, userID int64) error {
	var (
		expired  bool
		enrolled bool
	)

	err := DB.QueryRowContext(ctx,
		`SELECT (e.deadline < now()) AS expired,
		        EXISTS (
		            SELECT 1 FROM enrollments en
		            WHERE en.offering_id = e.offering_id AND en.user_id = $2
		        ) AS enrolled
		 FROM exercises e
		 WHERE e.id = $1 AND e.removed = FALSE`,
		exerciseID, userID,
	).Scan(&expired, &enrolled)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrExerciseNotFound
		}
		slog.ErrorContext(ctx, "error validating exercise",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if !enrolled {
		return ErrNotEnrolled
	}

	if expired {
		return ErrDeadlinePassed
	}

	return nil
}

/*
validateFileType checks the submitted file extension against the exercise's
allowed file types. Exercises without any configured allowed type are accepted
as-is (there is nothing to validate against).
*/
func validateFileType(ctx context.Context, exerciseID int64, filename string) error {
	rows, err := DB.QueryContext(ctx,
		`SELECT aft.extension
		 FROM exercises_allowed_file_types eaft
		 JOIN allowed_file_types aft ON aft.id = eaft.allowed_file_type_id
		 WHERE eaft.exercise_id = $1 AND aft.is_available = TRUE`,
		exerciseID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching allowed file types",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	defer rows.Close()

	allowed := make([]string, 0, 4)
	for rows.Next() {
		var extension string
		if err := rows.Scan(&extension); err != nil {
			slog.ErrorContext(ctx, "error scanning allowed file type",
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
		allowed = append(allowed, extension)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating allowed file types",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if len(allowed) == 0 {
		slog.WarnContext(ctx, "exercise has no allowed file types configured, accepting any file",
			slog.Int64("exercise_id", exerciseID),
		)
		return nil
	}

	for _, extension := range allowed {
		if filenameMatchesExtension(filename, extension) {
			return nil
		}
	}

	return ErrInvalidFileType
}

/*
filenameMatchesExtension reports whether filename ends with the given allowed
extension. Extensions are stored without a leading dot and may be compound
(e.g. "omp.c" matches "main.omp.c").
*/
func filenameMatchesExtension(filename, extension string) bool {
	name := strings.ToLower(strings.TrimSpace(filename))
	ext := strings.ToLower(strings.TrimSpace(extension))
	if ext == "" {
		return false
	}

	return name == ext || strings.HasSuffix(name, "."+ext)
}

/*
buildCommitKey builds the S3 object key ("<uuid>/<sanitized-basename>").
*/
func buildCommitKey(filename string) string {
	return uuid.NewString() + "/" + sanitizeFilename(filename)
}

/*
sanitizeFilename keeps only a safe basename so the key cannot escape its
prefix or contain characters that break the judge's language detection.
*/
func sanitizeFilename(filename string) string {
	base := strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	base = path.Base(base)
	if base == "" || base == "." || base == ".." || base == "/" {
		base = "submission"
	}

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	sanitized := strings.Trim(b.String(), ".")
	if sanitized == "" {
		return "submission"
	}
	return sanitized
}

/*
GetCommitSnapshot reads the current commit state and its test-case results.
*/
func GetCommitSnapshot(ctx context.Context, commitID int64) (*models.Snapshot, error) {
	commit := models.Commit{}

	err := DB.QueryRowContext(ctx,
		`SELECT id, user_id, exercise_id, status, num_correct_cases, score,
		        compiled, compilation_message, compilation_error,
		        compilation_started, compilation_finished, created_at,
		        s3_key, ip
		 FROM commits WHERE id = $1`,
		commitID,
	).Scan(
		&commit.ID, &commit.UserID, &commit.ExerciseID, &commit.Status,
		&commit.NumCorrectCases, &commit.Score, &commit.Compiled,
		&commit.CompilationMessage, &commit.CompilationError,
		&commit.CompilationStarted, &commit.CompilationFinished,
		&commit.CreatedAt, &commit.S3Key, &commit.IP,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommitNotFound
		}
		slog.ErrorContext(ctx, "error fetching commit",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err := DB.QueryContext(ctx,
		`SELECT exercise_test_case_id, cpu_time, mem_usage, user_output,
		        user_output_type, status, status_message, error_message
		 FROM commits_exercise_test_cases_results
		 WHERE commit_id = $1
		 ORDER BY exercise_test_case_id`,
		commitID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching commit results",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	results := make([]models.CaseResult, 0, 8)
	for rows.Next() {
		var result models.CaseResult
		if err := rows.Scan(
			&result.ExerciseTestCaseID, &result.CPUTime, &result.MemUsage,
			&result.UserOutput, &result.UserOutputType, &result.Status,
			&result.StatusMessage, &result.ErrorMessage,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning commit result",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating commit results",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return &models.Snapshot{
		Type:    "snapshot",
		Commit:  &commit,
		Results: results,
	}, nil
}

/*
ParseClientIP extracts a client IP from X-Forwarded-For (first entry) falling
back to RemoteAddr. Returns nil when neither holds a valid IP.
*/
func ParseClientIP(forwardedFor, remoteAddr string) *string {
	if forwardedFor != "" {
		first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			value := ip.String()
			return &value
		}
	}

	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		value := ip.String()
		return &value
	}

	return nil
}
