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

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
)

func ListMyExerciseSubmissions(
	ctx context.Context, exerciseID, userID int64,
) ([]models.SubmissionSummary, error) {
	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, status, num_correct_cases, score, created_at, s3_key
		 FROM commits
		 WHERE user_id = $1 AND exercise_id = $2
		   AND s3_key IS NOT NULL AND s3_key <> ''
		 ORDER BY created_at DESC, id DESC
		 LIMIT $3`,
		userID, exerciseID, SubmissionHistoryLimit,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error listing exercise submissions",
			slog.Int64("user_id", userID),
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	submissions := make([]models.SubmissionSummary, 0, 8)
	for rows.Next() {
		var (
			summary models.SubmissionSummary
			key     string
		)
		if err := rows.Scan(
			&summary.ID, &summary.Status, &summary.NumCorrectCases,
			&summary.Score, &summary.CreatedAt, &key,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning submission summary",
				slog.Int64("user_id", userID),
				slog.Int64("exercise_id", exerciseID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		summary.Filename = path.Base(key)
		submissions = append(submissions, summary)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating submission summaries",
			slog.Int64("user_id", userID),
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return submissions, nil
}

// CommitSource is a submitted source file opened for download or restoration.
type CommitSource struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

/*
VerifyMyCommit reports whether a commit belongs to the caller. Missing and
foreign commits both return ErrCommitNotFound, so the endpoint never discloses
whether someone else's commit exists.
*/
func VerifyMyCommit(ctx context.Context, commitID, userID int64) error {
	var id int64
	err := database.DB.QueryRowContext(ctx,
		`SELECT id FROM commits WHERE id = $1 AND user_id = $2`,
		commitID, userID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCommitNotFound
		}
		slog.ErrorContext(ctx, "error verifying commit ownership",
			slog.Int64("commit_id", commitID),
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	return nil
}

/*
GetMySubmissionSource opens the source of one of the caller's commits. The query
includes the user id, so another user's commit is indistinguishable from a
missing one and its existence is never disclosed.
*/
func GetMySubmissionSource(
	ctx context.Context, commitID, userID int64,
) (*CommitSource, error) {
	var key string
	err := database.DB.QueryRowContext(ctx,
		`SELECT s3_key
		 FROM commits
		 WHERE id = $1 AND user_id = $2
		   AND s3_key IS NOT NULL AND s3_key <> ''`,
		commitID, userID,
	).Scan(&key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommitNotFound
		}
		slog.ErrorContext(ctx, "error fetching submission source",
			slog.Int64("commit_id", commitID),
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	object, err := storage.GetCommitObject(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			slog.WarnContext(ctx, "submitted source object is missing",
				slog.Int64("commit_id", commitID),
				slog.String("s3_key", key),
			)
			return nil, ErrCommitSourceNotFound
		}
		slog.ErrorContext(ctx, "failed to open submitted source",
			slog.Int64("commit_id", commitID),
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return &CommitSource{
		Filename:    path.Base(key),
		ContentType: object.ContentType,
		Size:        object.Size,
		Body:        object.Body,
	}, nil
}

/*
GetCommitSnapshot reads the current commit state and its test-case results.
*/
func GetCommitSnapshot(ctx context.Context, commitID int64) (*models.Snapshot, error) {
	commit := models.Commit{}

	err := database.DB.QueryRowContext(ctx,
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

	rows, err := database.DB.QueryContext(ctx,
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
		Type:      "snapshot",
		Commit:    &commit,
		Results:   results,
		Artifacts: []models.SubmissionArtifact{},
	}, nil
}

/*
ParseClientIP normalizes the address the trusted-proxy middleware resolved,
falling back to RemoteAddr. The caller must pass the canonical client address
from the request context (middleware.GetClientIP), never a raw X-Forwarded-For
header: the client controls the start of that header, while the proxy-appended
hop is the only trustworthy entry.
*/
func ParseClientIP(clientIP, remoteAddr string) *string {
	if clientIP != "" {
		if ip := net.ParseIP(strings.TrimSpace(clientIP)); ip != nil {
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

/*
HiddenTestCaseIDs returns the ids of the cases whose program output the exercise
author chose not to release (`show_user_output = false`). It resolves a ghost
exercise to the real exercise that owns the test cases, matching the judge.
*/
func HiddenTestCaseIDs(
	ctx context.Context, commitID int64,
) (map[int64]struct{}, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT tc.id
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		JOIN exercises_test_cases tc
		  ON tc.exercise_id = CASE
		       WHEN e.ghost AND e.real_id IS NOT NULL THEN e.real_id
		       ELSE e.id
		     END
		WHERE c.id = $1 AND tc.show_user_output = FALSE`, commitID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching hidden test cases",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	hidden := make(map[int64]struct{})
	for rows.Next() {
		var caseID int64
		if err := rows.Scan(&caseID); err != nil {
			slog.ErrorContext(ctx, "error scanning hidden test case",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		hidden[caseID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating hidden test cases",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return hidden, nil
}

/*
RedactSnapshotOutput removes the program output of hidden cases from a snapshot
before it is sent to the submission's owner. Staff viewers keep the full result.
*/
func RedactSnapshotOutput(
	snapshot *models.Snapshot, hidden map[int64]struct{},
) {
	for i := range snapshot.Results {
		if _, ok := hidden[snapshot.Results[i].ExerciseTestCaseID]; !ok {
			continue
		}
		snapshot.Results[i].UserOutput = nil
		snapshot.Results[i].ErrorMessage = nil
	}
}
