package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// TestListUserExercises covers the home/exercises listing: it must return both
// open and already-closed exercises, and turn the caller's nullable progress
// columns into pointers the client can distinguish from zero.
func TestListUserExercises(t *testing.T) {
	mock := newMockDB(t)

	deadline := time.Date(2026, 11, 10, 23, 59, 0, 0, time.UTC)
	openedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	lastSubmittedAt := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`progress.best_score`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "offering_id", "offering_name", "title", "description",
			"deadline", "open_date", "is_open", "submission_count",
			"last_submission_at", "last_status", "best_score",
		}).
			AddRow(
				int64(15), int64(3), "CS101", "Hello, world", "…",
				deadline, openedAt, true, int64(2), lastSubmittedAt,
				"completed", 87.5,
			).
			AddRow(
				int64(16), int64(3), "CS101", "Closed exercise", "",
				deadline.Add(-24*time.Hour), openedAt, false, int64(0),
				nil, nil, nil,
			))

	exercises, err := ListUserExercises(context.Background(),
		map[string]any{"id": float64(7), "role": "student"})
	if err != nil {
		t.Fatalf("ListUserExercises: %v", err)
	}
	if len(exercises) != 2 {
		t.Fatalf("got %d exercises, want 2", len(exercises))
	}

	first := exercises[0]
	if first.ID != 15 || first.OfferingName != "CS101" ||
		first.Title != "Hello, world" || !first.IsOpen {
		t.Fatalf("first exercise = %+v", first)
	}
	if first.SubmissionCount != 2 || first.BestScore == nil ||
		*first.BestScore != 87.5 {
		t.Fatalf("first exercise progress = %+v", first)
	}
	if first.LastStatus == nil || *first.LastStatus != "completed" {
		t.Fatalf("last status = %v, want completed", first.LastStatus)
	}
	if first.LastSubmissionAt == nil ||
		!first.LastSubmissionAt.Equal(lastSubmittedAt) {
		t.Fatalf("last submission = %v, want %v",
			first.LastSubmissionAt, lastSubmittedAt)
	}

	second := exercises[1]
	if second.IsOpen || second.SubmissionCount != 0 ||
		second.LastStatus != nil || second.LastSubmissionAt != nil ||
		second.BestScore != nil {
		t.Fatalf("exercise without submissions = %+v", second)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListUserExercisesQueryFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`FROM exercises e`).
		WithArgs(int64(7)).
		WillReturnError(sql.ErrConnDone)

	_, err := ListUserExercises(context.Background(),
		map[string]any{"id": float64(7), "role": "student"})
	if !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListUserExercisesRequiresClaims(t *testing.T) {
	_, err := ListUserExercises(context.Background(), map[string]any{})
	if !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}
