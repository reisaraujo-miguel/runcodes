package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
)

// previewTTL bounds one preview session. It matches the normal session TTL:
// the admin keeps the preview alive by using the app, and an abandoned preview
// stops being a reduced-role session at the same pace as any other session.
const previewTTL = validation.SessionTTL

/*
authInfoFromClaims builds the session response, including the role an admin is
previewing when the session is a preview. The identity fields are exactly what
the token carries, so the UI shows the account whose permissions are being used.
*/
func authInfoFromClaims(
	claims map[string]any, expiresAt time.Time,
) models.AuthInfo {
	info := models.AuthInfo{
		ExpiresAt: expiresAt.Unix(),
	}

	// The claims map is decoded from a JWT on most paths, where numbers are
	// float64, but the preview handlers build it in memory with int values.
	// Accept both shapes.
	switch id := claims["id"].(type) {
	case float64:
		info.ID = int(id)
	case int:
		info.ID = id
	case int64:
		info.ID = int(id)
	}
	info.Name, _ = claims["name"].(string)
	info.Email, _ = claims["email"].(string)
	info.Role, _ = claims["role"].(string)
	info.PreviewingRole, _ = claims[services.PreviewClaim].(string)

	return info
}

/*
AdminStartPreview replaces the admin's session with one for the same account
using the requested role. It does not log in as, or read data from, any other
account: the user id, name, email and token version stay the administrator's,
and only the role claim changes. The admin panel, and every other admin-only
route, is therefore inaccessible until the preview ends.
*/
func AdminStartPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}
	adminID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	var req models.PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid preview request"},
		)
		return
	}

	admin, err := services.GetAdminForPreview(ctx, int64(adminID))
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	newClaims, err := services.PreviewClaims(admin, req.Role)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}
	jwtauth.SetIssuedAt(newClaims, time.Now())
	jwtauth.SetExpiryIn(newClaims, previewTTL)

	_, tokenString, err := validation.TokenAuth.Encode(newClaims)
	if err != nil {
		slog.ErrorContext(ctx, "error generating preview token",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	setSessionCookie(w, tokenString)

	slog.InfoContext(ctx, "admin started a role preview",
		slog.Int("admin_id", admin.ID),
		slog.String("preview_role", req.Role),
	)

	WriteResponse(w, http.StatusOK,
		authInfoFromClaims(newClaims, time.Now().Add(previewTTL)),
	)
}

/*
StopPreview restores the administrator's own role. The endpoint is reachable
from a preview session because it only accepts a token that carries the preview
claim; a regular student or professor session gets a 400.
*/
func StopPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}
	if _, previewing := claims[services.PreviewClaim].(string); !previewing {
		writeServiceError(ctx, w, services.ErrNotPreviewing)
		return
	}

	adminID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	admin, err := services.GetAdminForPreview(ctx, int64(adminID))
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	newClaims := map[string]any{
		"id":            admin.ID,
		"name":          admin.Name,
		"email":         admin.Email,
		"role":          admin.Role,
		"token_version": admin.TokenVersion,
	}
	jwtauth.SetIssuedAt(newClaims, time.Now())
	jwtauth.SetExpiryIn(newClaims, validation.SessionTTL)

	_, tokenString, err := validation.TokenAuth.Encode(newClaims)
	if err != nil {
		slog.ErrorContext(ctx, "error generating admin session token",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	setSessionCookie(w, tokenString)

	slog.InfoContext(ctx, "admin stopped a role preview",
		slog.Int("admin_id", admin.ID),
	)

	WriteResponse(w, http.StatusOK,
		authInfoFromClaims(newClaims, time.Now().Add(validation.SessionTTL)),
	)
}
