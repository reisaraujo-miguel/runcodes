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
			"id", "status", "num_correct_cases", "score", "created_at", "s3_key",
		}).
			AddRow(int64(9), "completed", 4, 100.0, created, "uuid/main.py").
			AddRow(int64(8), "compilation_error", 0, 0.0, created.Add(-time.Hour), "uuid/Main.java"))

	submissions, err := ListMyExerciseSubmissions(context.Background(), 10, 7)
	if err != nil {
		t.Fatalf("ListMyExerciseSubmissions: %v", err)
	}
	if len(submissions) != 2 {
		t.Fatalf("got %d submissions, want 2", len(submissions))
	}
	if submissions[0].ID != 9 || submissions[0].Filename != "main.py" {
		t.Fatalf("first submission = %+v, want #9 main.py", submissions[0])
	}
	if submissions[1].Filename != "Main.java" {
		t.Fatalf("second filename = %q, want Main.java", submissions[1].Filename)
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
