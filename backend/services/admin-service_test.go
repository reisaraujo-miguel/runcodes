package services

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// adminOfferingColumns is the projection queryAdminOfferings scans, in order.
var adminOfferingColumns = []string{
	"id", "name", "description", "end_date", "enrollment_code",
	"visible_to_enroll", "owner_id", "owner_name", "owner_email",
	"owner_org_id", "member_count", "exercise_count", "created_at",
}

/*
TestAdminListOfferingsAppliesPagination pins that the listing binds the page
bounds to the statement. A previous version passed limit and offset without
using them in the query, so every request failed with "got 3 parameters but the
statement requires 1".
*/
func TestAdminListOfferingsAppliesPagination(t *testing.T) {
	createdAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name      string
		query     string
		limit     int
		offset    int
		wantPat   string
		wantLimit int
		wantOff   int
	}{
		{
			name:      "search term keeps the requested page",
			query:     "  Turma  ",
			limit:     200,
			offset:    40,
			wantPat:   "%Turma%",
			wantLimit: 200,
			wantOff:   40,
		},
		{
			name:      "empty search uses the default page",
			query:     "",
			limit:     0,
			offset:    -10,
			wantPat:   "%%",
			wantLimit: DefaultAdminPageSize,
			wantOff:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockDB(t)
			mock.ExpectQuery(regexp.QuoteMeta("LIMIT $2 OFFSET $3")).
				WithArgs(tc.wantPat, tc.wantLimit, tc.wantOff).
				WillReturnRows(sqlmock.NewRows(adminOfferingColumns).AddRow(
					int64(1), "Turma", "Descrição", endDate, "ABCD",
					true, int64(7), "Professor", "prof@example.com", "1234567",
					int64(3), int64(2), createdAt,
				))

			offerings, err := AdminListOfferings(
				context.Background(), tc.query, tc.limit, tc.offset,
			)
			if err != nil {
				t.Fatalf("AdminListOfferings = %v, want nil", err)
			}
			if len(offerings) != 1 {
				t.Fatalf("len(offerings) = %d, want 1", len(offerings))
			}

			want := models.AdminOffering{
				ID:              1,
				Name:            "Turma",
				Description:     "Descrição",
				EndDate:         endDate,
				EnrollmentCode:  "ABCD",
				VisibleToEnroll: true,
				OwnerID:         ptrInt64(7),
				OwnerName:       "Professor",
				OwnerEmail:      "prof@example.com",
				MemberCount:     3,
				ExerciseCount:   2,
				CreatedAt:       createdAt,
			}
			got := offerings[0]
			if got.ID != want.ID || got.Name != want.Name ||
				got.OwnerName != want.OwnerName ||
				got.MemberCount != want.MemberCount ||
				got.ExerciseCount != want.ExerciseCount ||
				got.VisibleToEnroll != want.VisibleToEnroll {
				t.Fatalf("offering = %+v, want %+v", got, want)
			}
			if got.OwnerID == nil || *got.OwnerID != *want.OwnerID {
				t.Fatalf("owner_id = %v, want %d", got.OwnerID, *want.OwnerID)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

/*
TestAdminOfferingByIDBindsTheLookup pins the other queryAdminOfferings call
site: it reads one row by id but must still provide the page bounds.
*/
func TestAdminOfferingByIDBindsTheLookup(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("WHERE o.id = $1")).
		WithArgs(int64(7), 1, 0).
		WillReturnRows(sqlmock.NewRows(adminOfferingColumns).AddRow(
			int64(7), "Turma", "", time.Now(), "", true,
			nil, "", "", "", int64(0), int64(0), time.Now(),
		))

	offering, err := adminOfferingByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("adminOfferingByID = %v, want nil", err)
	}
	if offering.ID != 7 {
		t.Fatalf("id = %d, want 7", offering.ID)
	}
	if offering.OwnerID != nil {
		t.Fatalf("owner_id = %v, want nil", offering.OwnerID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminUpdateUserRejectsSelfEmailChange(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name, email, COALESCE\(org_id, ''\), role, confirmed, created_at`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "email", "org_id", "role", "confirmed", "created_at"},
		).AddRow(7, "Ada", "ada@example.com", "", "admin", true, time.Now()))

	_, err := AdminUpdateUser(context.Background(), 7, &models.UpdateUserRequest{
		Email: ptrString("new@example.com"),
	}, 7)
	if !errors.Is(err, ErrSelfManagement) {
		t.Fatalf("AdminUpdateUser = %v, want ErrSelfManagement", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerByEmail(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, name, email, role FROM users WHERE lower(email) = $1",
	)).
		WithArgs("prof@example.com").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "email", "role"},
		).AddRow(7, "Professor", "prof@example.com", "professor"))

	ownerID, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{
			OwnerEmail: ptrString("  Prof@Example.com "),
		},
	)
	if err != nil {
		t.Fatalf("resolveAdminOfferingOwner = %v, want nil", err)
	}
	if !ownerID.Valid || ownerID.Int64 != 7 {
		t.Fatalf("owner = %+v, want valid id 7", ownerID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerByOrgID(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, name, email, role FROM users WHERE org_id = $1",
	)).
		WithArgs("1234567").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "email", "role"},
		).AddRow(9, "Admin", "admin@example.com", "admin"))

	ownerID, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{
			OwnerOrgID: ptrString(" 1234567 "),
		},
	)
	if err != nil {
		t.Fatalf("resolveAdminOfferingOwner = %v, want nil", err)
	}
	if !ownerID.Valid || ownerID.Int64 != 9 {
		t.Fatalf("owner = %+v, want valid id 9", ownerID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerKeepsLegacyIDLookup(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, name, email, role FROM users WHERE id = $1",
	)).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "email", "role"},
		).AddRow(12, "Professor", "prof@example.com", "professor"))

	ownerID, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{OwnerID: ptrInt64(12)},
	)
	if err != nil {
		t.Fatalf("resolveAdminOfferingOwner = %v, want nil", err)
	}
	if !ownerID.Valid || ownerID.Int64 != 12 {
		t.Fatalf("owner = %+v, want valid id 12", ownerID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerRejectsAmbiguousReferences(t *testing.T) {
	mock := newMockDB(t)
	_, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{
			OwnerEmail: ptrString("prof@example.com"),
			OwnerOrgID: ptrString("1234567"),
		},
	)
	if !errors.Is(err, ErrOwnerReferenceConflict) {
		t.Fatalf("resolveAdminOfferingOwner = %v, want ErrOwnerReferenceConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerRequiresAPrivilegedAccount(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT id, name, email, role FROM users WHERE org_id = $1",
	)).
		WithArgs("7654321").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "email", "role"},
		).AddRow(3, "Student", "student@example.com", "student"))

	_, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{
			OwnerOrgID: ptrString("7654321"),
		},
	)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("resolveAdminOfferingOwner = %v, want ErrInvalidRole", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAdminOfferingOwnerRejectsEmptyReferences(t *testing.T) {
	mock := newMockDB(t)
	_, err := resolveAdminOfferingOwner(
		context.Background(),
		&models.AdminUpdateOfferingRequest{OwnerEmail: ptrString("  ")},
	)
	if !errors.Is(err, ErrOwnerReferenceEmpty) {
		t.Fatalf("resolveAdminOfferingOwner = %v, want ErrOwnerReferenceEmpty", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
