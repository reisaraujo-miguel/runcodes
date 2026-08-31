package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"runcodes/models"
	"runcodes/services"
	"runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
)

const debugModeEnv string = "DEBUG_MODE"

func SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.SignUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "invalid sign up request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	if err := validation.ValidateRequiredString(req.Name, 100); err != nil {
		slog.InfoContext(ctx, "someone tried to register with an invalid user name")
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := validation.ValidateEmailFormat(ctx, req.Email); err != nil {
		slog.InfoContext(ctx, "someone tried to register with an invalid email")
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	var err error
	if err = services.CheckEmailExistence(ctx, req.Email); err != nil {
		if errors.Is(err, services.ErrEmailExists) {
			slog.InfoContext(ctx,
				"someone tried to register an email that is already in use",
			)
			WriteResponse(w, http.StatusConflict,
				models.Error{Message: services.ErrEmailExists.Error()},
			)
		} else {
			slog.ErrorContext(ctx,
				"error while checking email",
				slog.String("error", err.Error()),
			)
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
		}
		return
	}

	if req.Password != req.PasswordConfirmation {
		slog.InfoContext(ctx, "someone tried to register with different passwords")
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "passwords don't match"},
		)
		return
	}

	if err := validation.ValidatePassword(req.Password); err != nil {
		slog.InfoContext(ctx, "someone tried to register with an invalid password")
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if err := services.SignUp(ctx, &req); err != nil {
		slog.ErrorContext(ctx,
			"error registering new user",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	WriteResponse(w, http.StatusCreated, nil)
}

func LogIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.LogInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid login request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	req.Email = strings.TrimSpace(req.Email)

	if err := validation.ValidateEmailFormat(ctx, req.Email); err != nil {
		slog.InfoContext(ctx, "someone tried to login with an invalid email")
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	var claims map[string]any
	var err error
	if claims, err = services.LogIn(ctx, &req); err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			slog.InfoContext(ctx,
				"someone tried to login with invalid credentials",
			)
			WriteResponse(w, http.StatusUnauthorized,
				models.Error{Message: services.ErrInvalidCredentials.Error()},
			)
		default:
			slog.ErrorContext(ctx,
				"error logging in user",
				slog.String("error", err.Error()),
			)
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
		}
		return
	}

	var tokenString string
	if _, tokenString, err = validation.TokenAuth.Encode(claims); err != nil {
		slog.ErrorContext(ctx,
			"error generating signed token string",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	setSessionCookie(w, tokenString)

	WriteResponse(w, http.StatusOK, nil)
}

/*
setSessionCookie writes the auth session cookie with the given token.
*/
func setSessionCookie(w http.ResponseWriter, tokenString string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		HttpOnly: true,                              // JS cannot access it
		Secure:   os.Getenv(debugModeEnv) != "true", // HTTPS only (disabled in local dev)
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   int(validation.SessionTTL.Seconds()),
		Expires:  time.Now().Add(validation.SessionTTL),
	})
}

/*
GetAuth returns the current session's user info (read from the JWT claims),
including the user role and the session expiry.
*/
func GetAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx,
			"error retrieving claims from context",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	id, okID := claims["id"].(float64)
	name, okName := claims["name"].(string)
	email, okEmail := claims["email"].(string)
	role, okRole := claims["role"].(string)
	if !okID || !okName || !okEmail || !okRole {
		slog.ErrorContext(ctx, "invalid claims in auth request")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// jwx decodes the registered "exp" claim as time.Time (unlike custom
	// claims, which come back as float64), so handle both shapes.
	var expiresAt int64
	switch exp := claims["exp"].(type) {
	case float64:
		expiresAt = int64(exp)
	case time.Time:
		expiresAt = exp.Unix()
	default:
		slog.ErrorContext(ctx, "invalid exp claim in auth request")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	WriteResponse(w, http.StatusOK, models.AuthInfo{
		User: models.User{
			ID:    int(id),
			Name:  name,
			Email: email,
			Role:  role,
		},
		ExpiresAt: expiresAt,
	})
}

/*
RefreshAuth issues a new session token for the current user, extending the
session (sliding expiration). The existing token must still be valid.
*/
func RefreshAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx,
			"error retrieving claims from context",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	id, okID := claims["id"].(float64)
	if !okID {
		slog.ErrorContext(ctx, "invalid claims in refresh request")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Re-fetch the user so role changes (or removed users) take effect on
	// refresh instead of trusting the stale claims in the old token.
	user, err := services.GetUserByID(ctx, int(id))
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			WriteResponse(w, http.StatusUnauthorized, nil)
			return
		}
		slog.ErrorContext(ctx,
			"error fetching user during session refresh",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	newClaims := map[string]any{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
	}
	jwtauth.SetIssuedAt(newClaims, time.Now())
	jwtauth.SetExpiryIn(newClaims, validation.SessionTTL)

	var tokenString string
	if _, tokenString, err = validation.TokenAuth.Encode(newClaims); err != nil {
		slog.ErrorContext(ctx,
			"error generating signed token string",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

	setSessionCookie(w, tokenString)

	WriteResponse(w, http.StatusOK, models.AuthInfo{
		User:      *user,
		ExpiresAt: time.Now().Add(validation.SessionTTL).Unix(),
	})
}
