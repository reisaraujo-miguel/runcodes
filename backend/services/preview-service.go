package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/models"
)

// Roles an administrator may preview. Previewing the admin role itself would
// not hide anything, so it is not offered.
const (
	PreviewRoleStudent   = "student"
	PreviewRoleProfessor = "professor"
)

// PreviewClaim marks a session as a role preview. Its value is the role the
// administrator asked to see; the account behind the session never changes.
const PreviewClaim = "preview_role"

// ValidPreviewRole reports whether role is one an admin may preview.
func ValidPreviewRole(role string) bool {
	return role == PreviewRoleStudent || role == PreviewRoleProfessor
}

/*
GetAdminForPreview loads the account that started or is maintaining a preview
session and verifies that it still holds the admin role. A demoted administrator
therefore cannot keep a preview alive.
*/
func GetAdminForPreview(
	ctx context.Context, adminID int64,
) (*models.User, error) {
	admin, err := GetUserByID(ctx, int(adminID))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if admin.Role != "admin" {
		return nil, ErrCannotPreview
	}
	return admin, nil
}

/*
PreviewClaims builds the session claims for previewing role on the admin's own
account. The user id, name, email and token version stay the admin's; only the
role changes, and the preview claim records that the change is a preview rather
than a real account role.
*/
func PreviewClaims(admin *models.User, role string) (map[string]any, error) {
	if admin.Role != "admin" {
		return nil, ErrCannotPreview
	}
	if !ValidPreviewRole(role) {
		return nil, ErrInvalidPreviewRole
	}

	return map[string]any{
		"id":            admin.ID,
		"name":          admin.Name,
		"email":         admin.Email,
		"role":          role,
		"token_version": admin.TokenVersion,
		PreviewClaim:    role,
	}, nil
}

/*
PreviewClaimsFromCurrent rebuilds a preview session from the current database
state, preserving the role the caller is previewing. It is used by the sliding
session refresh so a preview does not silently turn back into an admin session.
*/
func PreviewClaimsFromCurrent(
	ctx context.Context, adminID int64, role string,
) (map[string]any, error) {
	admin, err := GetAdminForPreview(ctx, adminID)
	if err != nil {
		slog.InfoContext(ctx, "ending role preview: account is no longer an admin",
			slog.Int64("user_id", adminID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	return PreviewClaims(admin, role)
}
