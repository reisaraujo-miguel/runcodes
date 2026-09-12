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

func TestClaimMissingS3Key(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "user_id", "exercise_id", "created_at", "s3_key"}).
		AddRow(int64(1), nil, int64(3), time.Now(), nil)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, user_id, exercise_id, created_at, s3_key").WillReturnRows(rows)
	mock.ExpectRollback()

	if _, err := NewWithDB(db).Claim(context.Background()); err == nil {
		t.Fatal("expected an error for a commit without an s3_key")
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
