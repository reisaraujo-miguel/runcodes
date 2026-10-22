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
		switch {
		case errors.Is(err, services.ErrInvalidVerificationToken):
			WriteResponse(w, http.StatusBadRequest,
				models.Error{Message: services.ErrInvalidVerificationToken.Error()},
			)
			return
		case errors.Is(err, services.ErrEmailExists):
			// The address was registered by someone else between the request and
			// the click. The pending change stays in place; a new request can
			// target a different address.
			WriteResponse(w, http.StatusConflict,
				models.Error{Message: "that email address is no longer available"},
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
RequireCurrentSession rejects a session whose account changed since the token was
issued: the email is unconfirmed, the role was changed, or the password/email was
changed and the account's token_version was bumped. Login already refuses an
unconfirmed account, but a session outlives the state it was issued in, and a
stateless JWT cannot be withdrawn without a server-side comparison.

The check is one indexed primary-key lookup per request, next to the queries the
handlers already run. Tokens issued before token_version existed have no claim
and are rejected, so a deployment of this change signs everyone out once.
*/
func RequireCurrentSession(next http.Handler) http.Handler {
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

		state, err := services.GetSessionState(ctx, userID)
		if err != nil {
			if errors.Is(err, services.ErrUserNotFound) {
				WriteResponse(w, http.StatusUnauthorized, nil)
				return
			}
			slog.ErrorContext(ctx, "error checking account session state",
				slog.Int("user_id", userID),
				slog.String("error", err.Error()),
			)
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
			return
		}

		claimVersion, ok := claims["token_version"].(float64)
		claimRole, _ := claims["role"].(string)
		if !ok || int(claimVersion) != state.TokenVersion || claimRole != state.Role {
			WriteResponse(w, http.StatusUnauthorized, nil)
			return
		}

		if !state.Confirmed {
			WriteResponse(w, http.StatusForbidden,
				models.Error{Message: services.ErrEmailNotVerified.Error()},
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}
