package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
AdminListUsers lists the platform's users, with an optional search term and
pagination (`?query=&limit=&offset=`).
*/
func AdminListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	users, err := services.AdminListUsers(
		ctx, r.URL.Query().Get("query"),
		queryInt(r, "limit"), queryInt(r, "offset"),
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, users)
}

/*
AdminUpdateUser changes any user of the platform: name, email, organization id,
role and confirmation.
*/
func AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
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

	userID, err := pathID(r, "id")
	if err != nil || userID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid user id"},
		)
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid user update request"},
		)
		return
	}

	user, err := services.AdminUpdateUser(ctx, int(userID), &req, adminID)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, user)
}

/*
AdminDeleteUser removes a user and everything that hangs off them.
*/
func AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
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

	userID, err := pathID(r, "id")
	if err != nil || userID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid user id"},
		)
		return
	}

	if err := services.AdminDeleteUser(ctx, int(userID), adminID); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
AdminListOfferings lists every class on the platform, with an optional search
term and pagination (`?query=&limit=&offset=`).
*/
func AdminListOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	offerings, err := services.AdminListOfferings(
		ctx, r.URL.Query().Get("query"),
		queryInt(r, "limit"), queryInt(r, "offset"),
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offerings)
}

/*
AdminUpdateOffering changes any class on the platform, including transferring it
to another professor.
*/
func AdminUpdateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	var req models.AdminUpdateOfferingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid offering update request"},
		)
		return
	}

	offering, err := services.AdminUpdateOffering(ctx, offeringID, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, offering)
}

/*
AdminDeleteOffering removes any class on the platform.
*/
func AdminDeleteOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	if err := services.AdminDeleteOffering(ctx, offeringID); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
AdminListOfferingMembers lists the members of any class, for the admin panel.
*/
func AdminListOfferingMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	members, err := services.AdminOfferingMembers(ctx, offeringID)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, members)
}

/*
queryInt reads an optional integer query parameter. A missing or unparsable value
is left to the service, which clamps it to its defaults.
*/
func queryInt(r *http.Request, name string) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.InfoContext(r.Context(), "ignoring malformed query parameter",
			slog.String("parameter", name),
			slog.String("value", raw),
		)
		return 0
	}

	return value
}
