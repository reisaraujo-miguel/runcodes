package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/runcodes-icmc/runcodes/database"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// newMockDB swaps the package-level database.DB for a sqlmock handle and
// restores the previous handle when the test finishes.
func newMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		db.Close()
	})

	return mock
}

// expectValidateExercise queues the authorization query used by validateExercise.
// The expected pattern includes the "NOT en.banned" clause so that dropping it
// (letting banned enrollments submit) fails the test.
func expectValidateExercise(mock sqlmock.Sqlmock, expired, enrolled bool) {
	mock.ExpectQuery(`AND NOT en\.banned`).
		WillReturnRows(sqlmock.NewRows([]string{"expired", "enrolled"}).
			AddRow(expired, enrolled))
}

func TestValidateExerciseAuthorization(t *testing.T) {
	tests := []struct {
		name     string
		expired  bool
		enrolled bool
		wantErr  error
	}{
		{"enrolled and open", false, true, nil},
		{"not enrolled", false, false, ErrNotEnrolled},
		{"banned enrollment", false, false, ErrNotEnrolled},
		{"deadline passed", true, true, ErrDeadlinePassed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockDB(t)
			expectValidateExercise(mock, tt.expired, tt.enrolled)

			err := validateExercise(context.Background(), 1, 2)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateExercise = %v, want %v", err, tt.wantErr)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateExerciseNotFound(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`AND NOT en\.banned`).WillReturnError(sql.ErrNoRows)

	if err := validateExercise(context.Background(), 1, 2); !errors.Is(err, ErrExerciseNotFound) {
		t.Fatalf("validateExercise = %v, want %v", err, ErrExerciseNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
