package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

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
	openDate time.Time, showBefore bool,
) {
	mock.ExpectQuery(`AND NOT en\.banned`).
		WillReturnRows(sqlmock.NewRows(
			[]string{"expired", "enrolled", "is_owner", "role", "open_date", "show_before_open_date"}).
			AddRow(expired, enrolled, owner, role, openDate, showBefore))
}

func TestValidateExerciseAuthorization(t *testing.T) {
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name       string
		expired    bool
		enrolled   bool
		owner      bool
		role       string
		openDate   time.Time
		showBefore bool
		wantErr    error
	}{
		{"enrolled and open", false, true, false, EnrollmentRoleStudent, time.Time{}, false, nil},
		{"not enrolled", false, false, false, "", time.Time{}, false, ErrNotEnrolled},
		{"banned enrollment", false, false, false, "", time.Time{}, false, ErrNotEnrolled},
		{"deadline passed for a student", true, true, false, EnrollmentRoleStudent, time.Time{}, false, ErrDeadlinePassed},
		{"deadline passed for a monitor", true, true, false, EnrollmentRoleMonitor, time.Time{}, false, ErrDeadlinePassed},
		// The owner and the professors assigned to the class teach it: the
		// deadline is a rule for the students who are being graded.
		{"deadline passed for the owner", true, false, true, "", time.Time{}, false, nil},
		{"deadline passed for a co-professor", true, true, false, EnrollmentRoleProfessor, time.Time{}, false, nil},
		// A future open date hides the exercise from students on every read path,
		// so submitting early must be refused too.
		{"hidden exercise for a student", false, true, false, EnrollmentRoleStudent, future, false, ErrExerciseNotFound},
		{"hidden exercise for a monitor", false, true, false, EnrollmentRoleMonitor, future, false, ErrExerciseNotFound},
		{"hidden exercise shown early", false, true, false, EnrollmentRoleStudent, future, true, nil},
		{"hidden exercise for the owner", false, false, true, "", future, false, nil},
		{"hidden exercise for a co-professor", false, true, false, EnrollmentRoleProfessor, future, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockDB(t)
			expectValidateExercise(mock, tt.expired, tt.enrolled, tt.owner, tt.role,
				tt.openDate, tt.showBefore)

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
