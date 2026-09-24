package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
GetPublicSettings returns the platform settings the client needs before anyone is
signed in: the contact address and the disclaimer shown on the login page. It is
a public route, so it never exposes anything but those two values.
*/
func GetPublicSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	settings, err := services.GetSettings(ctx)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, models.PlatformSettings{
		ContactEmail:          settings.ContactEmail,
		ContactDisclaimerHTML: settings.ContactDisclaimerHTML,
	})
}

/*
GetSettings returns the platform settings to an authenticated admin, which is
what the admin panel edits.
*/
func GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	settings, err := services.GetSettings(ctx)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, settings)
}

/*
UpdateSettings stores the contact address and the contact disclaimer the admin
panel submits.
*/
func UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	var req models.PlatformSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid settings request"},
		)
		return
	}

	req.ContactEmail = strings.TrimSpace(req.ContactEmail)

	settings, err := services.UpdateSettings(ctx, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	slog.InfoContext(ctx, "platform settings updated")

	WriteResponse(w, http.StatusOK, settings)
}
