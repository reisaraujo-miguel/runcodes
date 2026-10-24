package handlers

import (
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/services"
)

/*
TestAuthInfoFromClaims covers both claim shapes the API builds: a decoded JWT
(numbers are float64) and the in-memory map the preview handlers use before the
token is encoded (int). A preview response names the previewed role without
changing the account identity.
*/
func TestAuthInfoFromClaims(t *testing.T) {
	expires := time.Unix(1792873164, 0)

	t.Run("decoded preview token", func(t *testing.T) {
		info := authInfoFromClaims(map[string]any{
			"id":                  float64(1),
			"name":                "Admin",
			"email":               "admin@example.com",
			"role":                "student",
			services.PreviewClaim: "student",
		}, expires)

		if info.ID != 1 || info.Name != "Admin" || info.Role != "student" {
			t.Fatalf("user = %+v", info.User)
		}
		if info.PreviewingRole != "student" {
			t.Fatalf("previewing_role = %q, want student", info.PreviewingRole)
		}
		if info.ExpiresAt != expires.Unix() {
			t.Fatalf("expires_at = %d, want %d", info.ExpiresAt, expires.Unix())
		}
	})

	t.Run("in-memory preview claims", func(t *testing.T) {
		info := authInfoFromClaims(map[string]any{
			"id":                  1,
			"name":                "Admin",
			"email":               "admin@example.com",
			"role":                "professor",
			services.PreviewClaim: "professor",
		}, expires)

		if info.ID != 1 {
			t.Fatalf("id = %d, want 1", info.ID)
		}
		if info.PreviewingRole != "professor" {
			t.Fatalf("previewing_role = %q, want professor", info.PreviewingRole)
		}
	})

	t.Run("regular session", func(t *testing.T) {
		info := authInfoFromClaims(map[string]any{
			"id":    1,
			"name":  "Admin",
			"email": "admin@example.com",
			"role":  "admin",
		}, expires)

		if info.PreviewingRole != "" {
			t.Fatalf("previewing_role = %q, want empty", info.PreviewingRole)
		}
	})
}

/*
TestSessionRoleAllowed pins the session guard exception: only an admin account
may present a reduced preview role, and the preview claim must name exactly that
role. A normal session must always match the database role.
*/
func TestSessionRoleAllowed(t *testing.T) {
	tests := []struct {
		name         string
		claimRole    string
		databaseRole string
		previewRole  string
		want         bool
	}{
		{"normal student", "student", "student", "", true},
		{"normal admin", "admin", "admin", "", true},
		{"admin previewing student", "student", "admin", "student", true},
		{"admin previewing professor", "professor", "admin", "professor", true},
		{"admin role marked preview", "admin", "admin", "admin", false},
		{"preview claim mismatch", "student", "admin", "professor", false},
		{"preview without claim", "student", "admin", "", false},
		{"preview on professor account", "student", "professor", "student", false},
		{"real role mismatch", "professor", "student", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := sessionRoleAllowed(
				test.claimRole, test.databaseRole, test.previewRole,
			)
			if got != test.want {
				t.Fatalf("sessionRoleAllowed(%q, %q, %q) = %v, want %v",
					test.claimRole, test.databaseRole, test.previewRole,
					got, test.want)
			}
		})
	}
}
