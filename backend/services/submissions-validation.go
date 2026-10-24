package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"

	"github.com/google/uuid"
)

func checkSubmissionAdmission(ctx context.Context, userID int64) error {
	var pending int
	err := database.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM commits
		WHERE user_id = $1
		  AND status IN ('pending', 'queued', 'compiling', 'running')`,
		userID,
	).Scan(&pending)
	if err != nil {
		slog.ErrorContext(ctx, "error counting pending submissions",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if pending >= MaxPendingSubmissionsPerUser {
		return ErrSubmissionLimit
	}

	var queued int
	if err := database.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM commits WHERE status = 'queued'`,
	).Scan(&queued); err != nil {
		slog.ErrorContext(ctx, "error counting queued submissions",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if queued >= config.Get().Judge.MaxQueueDepth {
		return ErrQueueFull
	}

	return nil
}

/*
validateExercise checks the exercise exists and is not removed, that the user may
submit to it, and that its deadline has not passed.

A submission is allowed to an enrolled, non-banned participant. The class owner
and the professors assigned to it are the exception: they hold the class, so
they can submit to their own exercises without enrolling and without being
stopped by the deadline, which is what makes it possible to try an exercise
before handing it to the class (and to check a broken one after the deadline).
*/
func validateExercise(ctx context.Context, exerciseID, userID int64) error {
	var (
		expired    bool
		enrolled   bool
		isOwner    bool
		enrollRole string
		openDate   time.Time
		showBefore bool
	)

	err := database.DB.QueryRowContext(ctx,
		`SELECT (e.deadline < now()) AS expired,
		        EXISTS (
		            SELECT 1 FROM enrollments en
		            WHERE en.offering_id = e.offering_id AND en.user_id = $2
		              AND NOT en.banned
		        ) AS enrolled,
		        COALESCE(o.owner_id = $2, FALSE) AS is_owner,
		        COALESCE((
		            SELECT en.role::text FROM enrollments en
		            WHERE en.offering_id = e.offering_id AND en.user_id = $2
		              AND NOT en.banned
		        ), '') AS enroll_role,
		        e.open_date, e.show_before_open_date
		 FROM exercises e
		 JOIN offerings o ON o.id = e.offering_id
		 WHERE e.id = $1 AND e.removed = FALSE`,
		exerciseID, userID,
	).Scan(&expired, &enrolled, &isOwner, &enrollRole, &openDate, &showBefore)

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

	// Professors assigned to the class teach it: they are staff, not students.
	staff := isOwner || enrollRole == EnrollmentRoleProfessor

	if !enrolled && !isOwner {
		return ErrNotEnrolled
	}

	// Every read path hides an exercise until its open date (unless the author
	// chose to show it early); the submission path must apply the same rule, or a
	// student can submit early and read the hidden cases through the result stream.
	if !staff && !showBefore && openDate.After(nowFunc()) {
		return ErrExerciseNotFound
	}

	if expired && !staff {
		return ErrDeadlinePassed
	}

	return nil
}

/*
validateTestCases refuses a submission to an exercise with no test cases. Without
this check the judge would receive a run it cannot grade; the judge also treats
an empty case list as 0/uncompleted for commits queued before the check existed.
Ghost exercises have no cases of their own, so the count follows the same real
exercise the judge will use.
*/
func validateTestCases(ctx context.Context, exerciseID int64) error {
	var count int
	if err := database.DB.QueryRowContext(ctx, `
		SELECT count(*)
		FROM exercises e
		JOIN exercises_test_cases tc
		  ON tc.exercise_id = CASE
		       WHEN e.ghost AND e.real_id IS NOT NULL THEN e.real_id
		       ELSE e.id
		     END
		WHERE e.id = $1`, exerciseID,
	).Scan(&count); err != nil {
		slog.ErrorContext(ctx, "error counting exercise test cases",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if count == 0 {
		return ErrNoTestCases
	}
	return nil
}

/*
validateFileType checks the submitted file extension against the exercise's
allowed file types. Exercises without any configured allowed type are accepted
as-is (there is nothing to validate against).
*/
func validateFileType(ctx context.Context, exerciseID int64, filename string) error {
	rows, err := database.DB.QueryContext(ctx,
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
ListMyExerciseSubmissions returns the caller's submissions for an exercise,
newest first. Commits without a stored source are omitted because their code can
no longer be downloaded or restored.
*/
