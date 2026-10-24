package services

import (
	"errors"
	"testing"

	"github.com/runcodes-icmc/runcodes/models"
)

func adminUser() *models.User {
	return &models.User{
		ID: 1, Name: "Admin", Email: "admin@example.com",
		Role: "admin", TokenVersion: 7,
	}
}

func TestPreviewClaimsKeepTheAdminsOwnAccount(t *testing.T) {
	admin := adminUser()

	claims, err := PreviewClaims(admin, PreviewRoleStudent)
	if err != nil {
		t.Fatalf("PreviewClaims: %v", err)
	}

	if claims["id"] != 1 || claims["name"] != "Admin" ||
		claims["email"] != "admin@example.com" {
		t.Fatalf("preview must keep the admin identity: %#v", claims)
	}
	if claims["role"] != PreviewRoleStudent {
		t.Fatalf("role = %#v, want %q", claims["role"], PreviewRoleStudent)
	}
	if claims["token_version"] != 7 {
		t.Fatalf("token_version = %#v, want 7", claims["token_version"])
	}
	if claims[PreviewClaim] != PreviewRoleStudent {
		t.Fatalf("preview claim = %#v, want %q",
			claims[PreviewClaim], PreviewRoleStudent)
	}
}

func TestPreviewClaimsValidateTheRoleAndTheAccount(t *testing.T) {
	if _, err := PreviewClaims(adminUser(), "admin"); !errors.Is(err, ErrInvalidPreviewRole) {
		t.Fatalf("admin preview err = %v, want ErrInvalidPreviewRole", err)
	}
	if _, err := PreviewClaims(adminUser(), "monitor"); !errors.Is(err, ErrInvalidPreviewRole) {
		t.Fatalf("monitor preview err = %v, want ErrInvalidPreviewRole", err)
	}

	professor := &models.User{ID: 2, Role: "professor"}
	if _, err := PreviewClaims(professor, PreviewRoleStudent); !errors.Is(err, ErrCannotPreview) {
		t.Fatalf("professor err = %v, want ErrCannotPreview", err)
	}
}

func TestValidPreviewRole(t *testing.T) {
	for _, role := range []string{PreviewRoleStudent, PreviewRoleProfessor} {
		if !ValidPreviewRole(role) {
			t.Errorf("ValidPreviewRole(%q) = false", role)
		}
	}
	for _, role := range []string{"", "admin", "monitor", "STUDENT"} {
		if ValidPreviewRole(role) {
			t.Errorf("ValidPreviewRole(%q) = true", role)
		}
	}
}
