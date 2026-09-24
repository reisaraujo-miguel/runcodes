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
// The expected pattern includes the "NOT en.banned" clauses so that dropping one
// (letting banned enrollments submit) fails the test.
func expectValidateExercise(
	mock sqlmock.Sqlmock, expired, enrolled, owner bool, role string,
) {
	mock.ExpectQuery(`AND NOT en\.banned`).
		WillReturnRows(sqlmock.NewRows(
			[]string{"expired", "enrolled", "is_owner", "role"}).
			AddRow(expired, enrolled, owner, role))
}

func TestValidateExerciseAuthorization(t *testing.T) {
	tests := []struct {
		name     string
		expired  bool
		enrolled bool
		owner    bool
		role     string
		wantErr  error
	}{
		{"enrolled and open", false, true, false, EnrollmentRoleStudent, nil},
		{"not enrolled", false, false, false, "", ErrNotEnrolled},
		{"banned enrollment", false, false, false, "", ErrNotEnrolled},
		{"deadline passed for a student", true, true, false, EnrollmentRoleStudent, ErrDeadlinePassed},
		{"deadline passed for a monitor", true, true, false, EnrollmentRoleMonitor, ErrDeadlinePassed},
		// The owner and the professors assigned to the class teach it: the
		// deadline is a rule for the students who are being graded.
		{"deadline passed for the owner", true, false, true, "", nil},
		{"deadline passed for a co-professor", true, true, false, EnrollmentRoleProfessor, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockDB(t)
			expectValidateExercise(mock, tt.expired, tt.enrolled, tt.owner, tt.role)

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
