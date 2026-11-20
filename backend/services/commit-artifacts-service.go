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
CommitCaseIndex is a commit's frozen test-case numbering. Generated-file paths
name a case by that number (`test_v2_<number>/...`), while the results, the
artifact rows and the hidden-output rules use the platform-wide test-case id.
The number is frozen in the commit's limit snapshot when it is queued, so
deleting a test case later does not renumber the files of a stored run.
*/
type CommitCaseIndex struct {
	numberToID map[int]int64
	ids        map[int64]struct{}
}

// NewCommitCaseIndex builds an index from a frozen number -> id map. It exists
// so tests and callers that already hold the snapshot need no query.
func NewCommitCaseIndex(numberToID map[int]int64) CommitCaseIndex {
	index := CommitCaseIndex{
		numberToID: make(map[int]int64, len(numberToID)),
		ids:        make(map[int64]struct{}, len(numberToID)),
	}
	for number, id := range numberToID {
		index.numberToID[number] = id
		index.ids[id] = struct{}{}
	}

	return index
}

// IDForNumber resolves the frozen case number a generated file names.
func (i CommitCaseIndex) IDForNumber(number int) (int64, bool) {
	id, ok := i.numberToID[number]
	return id, ok
}

// HasID reports whether the case belongs to the commit at all.
func (i CommitCaseIndex) HasID(id int64) bool {
	_, ok := i.ids[id]
	return ok
}

/*
LoadCommitCaseIndex reads the numbering frozen in the commit's limit snapshot.
Deleting a test case removes its row but leaves the other numbers untouched, so
a stored file keeps pointing at the case that produced it.
*/
func LoadCommitCaseIndex(
	ctx context.Context, commitID int64,
) (CommitCaseIndex, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT case_number, exercise_test_case_id
		FROM commit_test_case_limits
		WHERE commit_id = $1`, commitID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching the commit's case numbering",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return CommitCaseIndex{}, ErrServer
	}
	defer rows.Close()

	pairs := make(map[int]int64, 8)
	for rows.Next() {
		var (
			number int
			id     int64
		)
		if err := rows.Scan(&number, &id); err != nil {
			slog.ErrorContext(ctx, "error scanning a commit case number",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			return CommitCaseIndex{}, ErrServer
		}
		pairs[number] = id
	}
	if err := rows.Err(); err != nil {
		return CommitCaseIndex{}, ErrServer
	}

	return NewCommitCaseIndex(pairs), nil
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
		SELECT a.exercise_test_case_id, a.state::text, a.reason,
		       a.limit_bytes, cl.case_number
		FROM commit_test_case_artifacts a
		LEFT JOIN commit_test_case_limits cl
		  ON cl.commit_id = a.commit_id
		 AND cl.exercise_test_case_id = a.exercise_test_case_id
		WHERE a.commit_id = $1
		ORDER BY a.exercise_test_case_id`, commitID)
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
			number sql.NullInt64
		)
		if err := rows.Scan(&id, &state, &reason, &limit, &number); err != nil {
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
		if number.Valid {
			value := number.Int64
			artifact.Number = &value
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
