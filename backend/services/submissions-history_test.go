package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestListMyExerciseSubmissions(t *testing.T) {
	mock := newMockDB(t)
	created := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`WHERE user_id = \$1 AND exercise_id = \$2`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "number", "status", "num_correct_cases", "score",
			"created_at", "s3_key",
		}).
			AddRow(int64(9), int64(2), "completed", 4, 100.0, created, "uuid/main.py").
			AddRow(int64(8), int64(1), "compilation_error", 0, 0.0, created.Add(-time.Hour), "uuid/Main.java"))

	submissions, err := ListMyExerciseSubmissions(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("ListMyExerciseSubmissions: %v", err)
	}
	if len(submissions) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submissions))
	}
	if submissions[0].ID != 9 || submissions[0].Number != 2 ||
		submissions[0].Filename != "main.py" {
		t.Fatalf("first submission = %+v, want #2 (#9) main.py", submissions[0])
	}
	if submissions[1].Number != 1 || submissions[1].Filename != "Main.java" {
		t.Fatalf("second submission = %+v, want #1 Main.java", submissions[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListMyExerciseSubmissionsErrors(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`WHERE user_id = \$1 AND exercise_id = \$2`).
		WillReturnError(sql.ErrConnDone)

	if _, err := ListMyExerciseSubmissions(context.Background(), 10, 7); !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}

func TestSubmissionNumber(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT count\(\*\)`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(10)))

	number, err := SubmissionNumber(context.Background(), 42)
	if err != nil {
		t.Fatalf("SubmissionNumber: %v", err)
	}
	if number != 10 {
		t.Fatalf("number = %d, want 10", number)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSubmissionNumberError(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT count\(\*\)`).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrConnDone)

	if _, err := SubmissionNumber(context.Background(), 42); !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}

func TestGetCommitSnapshotIncludesSubmissionNumber(t *testing.T) {
	mock := newMockDB(t)
	created := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM commits c WHERE c.id = \$1`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "number", "user_id", "exercise_id", "status",
			"num_correct_cases", "score", "compiled", "compilation_message",
			"compilation_error", "compilation_started", "compilation_finished",
			"created_at", "s3_key", "ip",
		}).AddRow(
			int64(42), int64(10), int64(7), int64(15), "completed",
			5, 100.0, true, nil, nil, nil, nil, created, "uuid/main.c", nil,
		))
	mock.ExpectQuery(`FROM commits_exercise_test_cases_results`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"exercise_test_case_id", "cpu_time", "wall_time", "mem_usage",
			"user_output", "user_output_type", "status", "reason",
			"status_message", "error_message",
		}))
	mock.ExpectQuery(`FROM commit_test_case_artifacts`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"exercise_test_case_id", "state", "reason", "limit_bytes",
		}))

	snapshot, err := GetCommitSnapshot(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetCommitSnapshot: %v", err)
	}
	if snapshot.Commit.Number != 10 {
		t.Fatalf("Number = %d, want 10", snapshot.Commit.Number)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMyCommit(t *testing.T) {
	t.Run("owned commit", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT id FROM commits WHERE id = \$1 AND user_id = \$2`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

		if err := VerifyMyCommit(context.Background(), 42, 7); err != nil {
			t.Fatalf("VerifyMyCommit: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing or foreign commit", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT id FROM commits WHERE id = \$1 AND user_id = \$2`).
			WillReturnError(sql.ErrNoRows)

		if err := VerifyMyCommit(context.Background(), 42, 7); !errors.Is(err, ErrCommitNotFound) {
			t.Fatalf("err = %v, want ErrCommitNotFound", err)
		}
	})

	t.Run("database failure", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT id FROM commits WHERE id = \$1 AND user_id = \$2`).
			WillReturnError(sql.ErrConnDone)

		if err := VerifyMyCommit(context.Background(), 42, 7); !errors.Is(err, ErrServer) {
			t.Fatalf("err = %v, want ErrServer", err)
		}
	})
}

func TestGetMySubmissionSourceOwnership(t *testing.T) {
	// The user id is part of the lookup, so a commit belonging to someone else
	// is indistinguishable from a missing one and its existence is not disclosed.
	mock := newMockDB(t)
	mock.ExpectQuery(`WHERE id = \$1 AND user_id = \$2`).
		WillReturnError(sql.ErrNoRows)

	_, err := GetMySubmissionSource(context.Background(), 42, 7)
	if !errors.Is(err, ErrCommitNotFound) {
		t.Fatalf("err = %v, want ErrCommitNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetMySubmissionSourceDatabaseFailure(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`WHERE id = \$1 AND user_id = \$2`).
		WillReturnError(sql.ErrConnDone)

	_, err := GetMySubmissionSource(context.Background(), 42, 7)
	if !errors.Is(err, ErrServer) {
		t.Fatalf("err = %v, want ErrServer", err)
	}
}
