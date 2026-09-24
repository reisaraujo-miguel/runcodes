package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// expectEnrollmentLookup queues the single query Enroll runs to find the class a
// code belongs to, with the caller's existing enrollment (if any).
func expectEnrollmentLookup(
	mock sqlmock.Sqlmock, ownerID any, visible, enrolled, banned bool, role string,
) {
	mock.ExpectQuery(`FROM offerings o`).WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "name", "description", "end_date", "owner_id",
			"visible_to_enroll", "role", "banned", "enrolled", "owner_name",
		}).AddRow(
			int64(3), "Turma", "", time.Now().Add(time.Hour), ownerID,
			visible, role, banned, enrolled, "Professor",
		))
}

/*
TestEnrollRules pins who may join a class and why a join is refused. The code is
the only credential a student has, so every refusal here is a rule the class owner
relies on.
*/
func TestEnrollRules(t *testing.T) {
	claims := map[string]any{"id": float64(2), "role": "student"}

	t.Run("joins an open class", func(t *testing.T) {
		mock := newMockDB(t)
		expectEnrollmentLookup(mock, int64(7), true, false, false, "")
		mock.ExpectExec(`INSERT INTO enrollments`).
			WillReturnResult(sqlmock.NewResult(0, 1))

		enrollment, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "abcd"}, claims)
		if err != nil {
			t.Fatalf("Enroll = %v, want nil", err)
		}
		if enrollment.Role != EnrollmentRoleStudent {
			t.Fatalf("role = %q, want %q", enrollment.Role, EnrollmentRoleStudent)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("already enrolled is idempotent", func(t *testing.T) {
		mock := newMockDB(t)
		// No INSERT is queued: a second use of the same code must not write.
		expectEnrollmentLookup(mock, int64(7), true, true, false, EnrollmentRoleStudent)

		if _, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "ABCD"}, claims); err != nil {
			t.Fatalf("Enroll = %v, want nil", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("closed enrollment", func(t *testing.T) {
		mock := newMockDB(t)
		expectEnrollmentLookup(mock, int64(7), false, false, false, "")

		_, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "ABCD"}, claims)
		if !errors.Is(err, ErrEnrollmentClosed) {
			t.Fatalf("Enroll = %v, want ErrEnrollmentClosed", err)
		}
	})

	t.Run("banned student cannot enroll", func(t *testing.T) {
		mock := newMockDB(t)
		expectEnrollmentLookup(mock, int64(7), true, true, true, EnrollmentRoleStudent)

		_, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "ABCD"}, claims)
		if !errors.Is(err, ErrEnrollmentBanned) {
			t.Fatalf("Enroll = %v, want ErrEnrollmentBanned", err)
		}
	})

	t.Run("the owner does not enroll in their own class", func(t *testing.T) {
		mock := newMockDB(t)
		expectEnrollmentLookup(mock, int64(2), true, false, false, "")

		_, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "ABCD"}, claims)
		if !errors.Is(err, ErrOfferingOwned) {
			t.Fatalf("Enroll = %v, want ErrOfferingOwned", err)
		}
	})

	t.Run("unknown code", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`FROM offerings o`).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		_, err := Enroll(context.Background(),
			&models.EnrollRequest{EnrollmentCode: "ZZZZ"}, claims)
		if !errors.Is(err, ErrInvalidEnrollmentCode) {
			t.Fatalf("Enroll = %v, want ErrInvalidEnrollmentCode", err)
		}
	})
}

/*
TestUnenrollKeepsBannedEnrollments pins the ban: it lives on the enrollment row,
so a banned student must not be able to delete the row and walk back in with the
same enrollment code.
*/
func TestUnenrollKeepsBannedEnrollments(t *testing.T) {
	claims := map[string]any{"id": float64(2), "role": "student"}

	t.Run("a banned enrollment is kept", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT owner_id FROM offerings`).
			WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(7)))
		// The deletion excludes banned rows; removing that clause fails this test.
		mock.ExpectExec(`DELETE FROM enrollments\s+WHERE user_id = \$1 AND offering_id = \$2 AND NOT banned`).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT banned FROM enrollments`).
			WillReturnRows(sqlmock.NewRows([]string{"banned"}).AddRow(true))

		if err := Unenroll(context.Background(), 3, claims); !errors.Is(err, ErrEnrollmentBanned) {
			t.Fatalf("Unenroll = %v, want ErrEnrollmentBanned", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a student who is not enrolled", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT owner_id FROM offerings`).
			WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(7)))
		mock.ExpectExec(`DELETE FROM enrollments`).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT banned FROM enrollments`).
			WillReturnRows(sqlmock.NewRows([]string{"banned"}))

		if err := Unenroll(context.Background(), 3, claims); !errors.Is(err, ErrNotEnrolled) {
			t.Fatalf("Unenroll = %v, want ErrNotEnrolled", err)
		}
	})

	t.Run("the owner cannot unenroll", func(t *testing.T) {
		mock := newMockDB(t)
		mock.ExpectQuery(`SELECT owner_id FROM offerings`).
			WillReturnRows(sqlmock.NewRows([]string{"owner_id"}).AddRow(int64(2)))

		if err := Unenroll(context.Background(), 3, claims); !errors.Is(err, ErrOwnerCannotUnenroll) {
			t.Fatalf("Unenroll = %v, want ErrOwnerCannotUnenroll", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}
