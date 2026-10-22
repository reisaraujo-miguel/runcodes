package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"
)

/*
ForgotPassword emails a password reset link. The response is always 202 whether
or not the address has an account, so the route cannot be used to enumerate
users. A delivery or database failure is logged, not surfaced, for the same
reason.
*/
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid password reset request"},
		)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if err := validation.ValidateEmailFormat(ctx, req.Email); err != nil {
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := services.RequestPasswordReset(ctx, req.Email); err != nil {
		slog.ErrorContext(ctx, "error sending password reset email",
			slog.String("error", err.Error()),
		)
	}

	w.WriteHeader(http.StatusAccepted)
}

/*
ResetPassword consumes a recovery token and stores the new password. The token
is single use, and an unknown, consumed or expired token gets the same message
so the link's state is not disclosed.
*/
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid password reset request"},
		)
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "password reset token is required"},
		)
		return
	}

	if req.Password != req.PasswordConfirmation {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "passwords don't match"},
		)
		return
	}

	if err := validation.ValidatePassword(req.Password); err != nil {
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := services.ResetPassword(ctx, req.Token, req.Password); err != nil {
		if errors.Is(err, services.ErrInvalidPasswordResetToken) {
			WriteResponse(w, http.StatusBadRequest,
				models.Error{Message: services.ErrInvalidPasswordResetToken.Error()},
			)
			return
		}
		slog.ErrorContext(ctx, "error resetting password",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
