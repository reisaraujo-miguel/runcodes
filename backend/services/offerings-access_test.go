package services

import (
	"context"
	"strings"
	"testing"

	"github.com/runcodes-icmc/runcodes/models"
)

/*
TestOfferingRelationPermissions pins the class authorization matrix. It is the
only place the "may this user touch this class?" question is answered, so a
change here changes every authoring and member endpoint at once.
*/
func TestOfferingRelationPermissions(t *testing.T) {
	tests := []struct {
		name      string
		relation  offeringRelation
		wantView  bool
		wantWrite bool
	}{
		{
			name:      "owner",
			relation:  offeringRelation{Owner: true},
			wantView:  true,
			wantWrite: true,
		},
		{
			name:      "admin panel",
			relation:  offeringRelation{AdminOverride: true},
			wantView:  true,
			wantWrite: true,
		},
		{
			name:      "professor assigned to the class",
			relation:  offeringRelation{Role: EnrollmentRoleProfessor},
			wantView:  true,
			wantWrite: true,
		},
		{
			name:      "monitor",
			relation:  offeringRelation{Role: EnrollmentRoleMonitor},
			wantView:  true,
			wantWrite: false,
		},
		{
			name:      "student",
			relation:  offeringRelation{Role: EnrollmentRoleStudent},
			wantView:  true,
			wantWrite: false,
		},
		{
			name:      "banned professor",
			relation:  offeringRelation{Role: EnrollmentRoleProfessor, Banned: true},
			wantView:  false,
			wantWrite: false,
		},
		{
			name:      "banned student",
			relation:  offeringRelation{Role: EnrollmentRoleStudent, Banned: true},
			wantView:  false,
			wantWrite: false,
		},
		{
			name:      "unrelated user",
			relation:  offeringRelation{},
			wantView:  false,
			wantWrite: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.relation.CanView(); got != tc.wantView {
				t.Errorf("CanView = %v, want %v", got, tc.wantView)
			}
			if got := tc.relation.CanAuthor(); got != tc.wantWrite {
				t.Errorf("CanAuthor = %v, want %v", got, tc.wantWrite)
			}
		})
	}
}

/*
TestValidAssignableEnrollmentRole keeps 'student' out of the roles an owner can
hand out: students join with the enrollment code, and an endpoint that assigns
the role would be a way to put somebody in a class without their code.
*/
func TestValidAssignableEnrollmentRole(t *testing.T) {
	valid := []string{EnrollmentRoleMonitor, EnrollmentRoleProfessor}
	for _, role := range valid {
		if !ValidAssignableEnrollmentRole(role) {
			t.Errorf("ValidAssignableEnrollmentRole(%q) = false, want true", role)
		}
	}

	invalid := []string{EnrollmentRoleStudent, "", "admin", "PROFESSOR"}
	for _, role := range invalid {
		if ValidAssignableEnrollmentRole(role) {
			t.Errorf("ValidAssignableEnrollmentRole(%q) = true, want false", role)
		}
	}
}

/*
TestNormalizePage keeps an admin listing bounded: a missing or absurd limit must
never turn into an unbounded query.
*/
func TestNormalizePage(t *testing.T) {
	tests := []struct {
		limit, offset         int
		wantLimit, wantOffset int
	}{
		{0, 0, DefaultAdminPageSize, 0},
		{-5, -5, DefaultAdminPageSize, 0},
		{10, 20, 10, 20},
		{MaxAdminPageSize + 1, 0, MaxAdminPageSize, 0},
	}

	for _, tc := range tests {
		limit, offset := normalizePage(tc.limit, tc.offset)
		if limit != tc.wantLimit || offset != tc.wantOffset {
			t.Errorf("normalizePage(%d, %d) = (%d, %d), want (%d, %d)",
				tc.limit, tc.offset, limit, offset, tc.wantLimit, tc.wantOffset)
		}
	}
}

/*
TestValidateSettings rejects settings that would leave the login page without a
usable contact address. An empty disclaimer is valid: it is how an admin hides
it.
*/
func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name    string
		request models.PlatformSettings
		wantErr bool
	}{
		{
			name: "complete settings",
			request: models.PlatformSettings{
				ContactEmail:          "suporte@example.com",
				ContactDisclaimerHTML: "Fale com <a href=\"mailto:suporte@example.com\">nós</a>.",
			},
		},
		{
			name: "disclaimer cleared on purpose",
			request: models.PlatformSettings{
				ContactEmail: "suporte@example.com",
			},
		},
		{
			name: "disclaimer too long",
			request: models.PlatformSettings{
				ContactEmail:          "suporte@example.com",
				ContactDisclaimerHTML: strings.Repeat("a", MaxContactDisclaimerLength+1),
			},
			wantErr: true,
		},
		{
			name: "missing email",
			request: models.PlatformSettings{
				ContactDisclaimerHTML: "Fale conosco.",
			},
			wantErr: true,
		},
		{
			name: "malformed email",
			request: models.PlatformSettings{
				ContactEmail:          "not-an-email",
				ContactDisclaimerHTML: "Fale conosco.",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSettings(context.Background(), &tc.request)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSettings = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}
