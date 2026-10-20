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
VerifyEmail consumes the token from a verification email and confirms the
account. It is public: the user follows the link from their inbox, possibly in a
browser that is not signed in.
*/
func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid verification request"},
		)
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "verification token is required"},
		)
		return
	}

	if err := services.VerifyEmail(ctx, req.Token); err != nil {
		if errors.Is(err, services.ErrInvalidVerificationToken) {
			WriteResponse(w, http.StatusBadRequest,
				models.Error{Message: services.ErrInvalidVerificationToken.Error()},
			)
			return
		}
		slog.ErrorContext(ctx, "error verifying email",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/*
ResendVerification sends a new verification link. The response is the same
whether or not the address has an account, so the endpoint cannot be used to
enumerate users; the status says only that the request was accepted.
*/
func ResendVerification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid verification request"},
		)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if err := validation.ValidateEmailFormat(ctx, req.Email); err != nil {
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := services.ResendVerification(ctx, req.Email); err != nil {
		// Logged, not surfaced: a delivery failure must not tell the caller
		// whether the address is registered.
		slog.ErrorContext(ctx, "error resending verification email",
			slog.String("error", err.Error()),
		)
	}

	w.WriteHeader(http.StatusAccepted)
}

/*
RequireVerifiedEmail rejects a session whose account has not confirmed its email.

Login already refuses an unconfirmed account, but a session outlives the state it
was issued in: an admin can uncheck an account, or a migrated row can be
unconfirmed. Checking the database here makes "verified" a property of the
account rather than of the token. It is one indexed primary-key lookup per
request, next to the queries the handlers already run.
*/
func RequireVerifiedEmail(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		confirmed, err := services.IsEmailVerified(ctx, userID)
		if err != nil {
			if errors.Is(err, services.ErrUserNotFound) {
				WriteResponse(w, http.StatusUnauthorized, nil)
				return
			}
			slog.ErrorContext(ctx, "error checking account verification",
				slog.Int("user_id", userID),
				slog.String("error", err.Error()),
			)
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
			return
		}
		if !confirmed {
			WriteResponse(w, http.StatusForbidden,
				models.Error{Message: services.ErrEmailNotVerified.Error()},
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}
