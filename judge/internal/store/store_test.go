package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestClaimEmptyQueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	commit, err := NewWithDB(db).Claim(context.Background())
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if commit != nil {
		t.Fatalf("want no commit, got %+v", commit)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "user_id", "exercise_id", "created_at", "s3_key"}).
		AddRow(int64(42), int64(7), int64(3), time.Now(), "bucket/main.c")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").WillReturnRows(rows)
	mock.ExpectExec("UPDATE commits SET status").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT CASE WHEN ghost").
		WillReturnRows(sqlmock.NewRows([]string{"real_id"}).AddRow(int64(5)))
	mock.ExpectCommit()

	commit, err := NewWithDB(db).Claim(context.Background())
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if commit.ID != 42 || commit.UserID != 7 || commit.ExerciseID != 3 || commit.RealExerciseID != 5 {
		t.Fatalf("unexpected commit: %+v", commit)
	}
	if commit.S3Key != "bucket/main.c" || commit.Filename() != "main.c" {
		t.Fatalf("unexpected key/filename: %q / %q", commit.S3Key, commit.Filename())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimMissingS3KeyIsMarkedTerminal(t *testing.T) {
	tests := []struct {
		name string
		key  any
	}{
		{"null s3_key", nil},
		{"empty s3_key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"id", "user_id", "exercise_id", "created_at", "s3_key"}).
				AddRow(int64(1), nil, int64(3), time.Now(), tt.key)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").WillReturnRows(rows)
			// The malformed row must be marked terminal and committed, not rolled
			// back, so it is not re-selected forever and later submissions are not
			// starved.
			mock.ExpectExec("UPDATE commits SET status = 'server_error'").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			commit, err := NewWithDB(db).Claim(context.Background())
			if err != nil {
				t.Fatalf("Claim: %v", err)
			}
			if commit != nil {
				t.Fatalf("want no commit, got %+v", commit)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestClaimMissingExerciseIsMarkedTerminal covers the commit that can never be
// graded because its exercise is gone (or NULL). Like the missing-s3_key case it
// must settle in the claim transaction: returning an error would roll back and
// leave the row queued, and every poll takes the oldest row first, so the same
// row would be selected forever and starve the submissions behind it.
func TestClaimMissingExerciseIsMarkedTerminal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "user_id", "exercise_id", "created_at", "s3_key"}).
		AddRow(int64(9), nil, int64(404), time.Now(), "bucket/main.c")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").WillReturnRows(rows)
	mock.ExpectExec("UPDATE commits SET status = 'compiling'").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT CASE WHEN ghost").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("UPDATE commits SET status = 'server_error'").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	commit, err := NewWithDB(db).Claim(context.Background())
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if commit != nil {
		t.Fatalf("want no commit, got %+v", commit)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClaimDatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	if _, err := NewWithDB(db).Claim(context.Background()); err == nil {
		t.Fatal("expected the database error to propagate")
	}
}
