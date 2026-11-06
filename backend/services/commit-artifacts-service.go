package services

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
Generated-file paths name a case by its position inside the exercise
(`test_<number>/...`). The database, including the hidden-output rules, uses the
platform-wide test-case id. CommitTestCaseIDs returns the ids in that numbering
order, so the two representations can be translated without a query per file.
*/
func CommitTestCaseIDs(ctx context.Context, commitID int64) ([]int64, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT tc.id
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		JOIN commit_test_case_limits cl ON cl.commit_id = c.id
		JOIN exercises_test_cases tc
		  ON tc.id = cl.exercise_test_case_id
		 AND tc.exercise_id =
		     CASE WHEN e.ghost AND e.real_id IS NOT NULL
		          THEN e.real_id ELSE e.id END
		WHERE c.id = $1
		ORDER BY tc.id`, commitID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching the commit's test cases",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	ids := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			slog.ErrorContext(ctx, "error scanning a commit test case",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	return ids, nil
}

/*
CommitExerciseTitle returns the title of the exercise that produced the commit.
This is used to name the zip file when downloading a submission.
*/
func CommitExerciseTitle(ctx context.Context, commitID int64) (string, error) {
	var title string
	err := database.DB.QueryRowContext(ctx, `
		SELECT e.title
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		WHERE c.id = $1`, commitID).Scan(&title)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching the commit's exercise title",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}
	return title, nil
}

/*
ListCommitArtifacts returns the upload state of each test case that produced
generated files. The rows are written from the judge's artifact events before
they are relayed, so a snapshot can explain a missing download after a reload.
*/
func ListCommitArtifacts(
	ctx context.Context, commitID int64,
) ([]models.SubmissionArtifact, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT exercise_test_case_id, state::text, reason, limit_bytes
		FROM commit_test_case_artifacts
		WHERE commit_id = $1
		ORDER BY exercise_test_case_id`, commitID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching the commit's artifacts",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	artifacts := make([]models.SubmissionArtifact, 0, 8)
	for rows.Next() {
		var (
			id     int64
			state  string
			reason sql.NullString
			limit  sql.NullInt64
		)
		if err := rows.Scan(&id, &state, &reason, &limit); err != nil {
			slog.ErrorContext(ctx, "error scanning a commit artifact",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		testCaseID := id
		artifact := models.SubmissionArtifact{
			Kind:       "output",
			TestCaseID: &testCaseID,
			State:      state,
		}
		if reason.Valid {
			artifact.Reason = reason.String
		}
		if limit.Valid {
			value := limit.Int64
			artifact.LimitBytes = &value
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	return artifacts, nil
}

/*
SettlePendingArtifacts marks an upload that was still pending when the run
reached a terminal status as failed. Nothing will report the upload's outcome
after the terminal event, so leaving it pending would show a spinner that can
never finish.
*/
func SettlePendingArtifacts(status string, artifacts []models.SubmissionArtifact) {
	if !judge.IsTerminalStatus(status) {
		return
	}

	for i := range artifacts {
		if artifacts[i].State == "pending" {
			artifacts[i].State = "failed"
			artifacts[i].Reason = "upload_interrupted"
		}
	}
}
