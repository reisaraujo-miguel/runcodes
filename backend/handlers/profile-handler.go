package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"
)

/*
GetProfile returns the caller's own account.
*/
func GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	profile, err := services.GetProfile(ctx, userID)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, profile)
}

/*
UpdateProfile updates the caller's own name, email and organization id.
*/
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid profile update request"},
		)
		return
	}

	profile, err := services.UpdateProfile(ctx, userID, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, profile)
}

/*
ChangePassword replaces the caller's password after checking the current one.
The session cookie is left in place: it is a stateless token, so the client keeps
working with it until it expires.
*/
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "Invalid password change request"},
		)
		return
	}

	if req.NewPassword != req.NewPasswordConfirmation {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "passwords don't match"},
		)
		return
	}

	if err := validation.ValidatePassword(req.NewPassword); err != nil {
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := services.ChangePassword(ctx, userID, &req); err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			WriteResponse(w, http.StatusUnauthorized,
				models.Error{Message: "current password is incorrect"},
			)
			return
		}
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
