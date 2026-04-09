package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"runcodes/models"
	"runcodes/services"
	"runcodes/validation"
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
		if err == services.ErrEmailExists {
			slog.InfoContext(ctx, "someone tried to register an email that is already in use")
			WriteResponse(w, http.StatusConflict, models.Error{Message: err.Error()})
		} else {
			slog.ErrorContext(ctx, "error while checking email", slog.String("error", err.Error()))
			WriteResponse(w, http.StatusInternalServerError, models.Error{Message: err.Error()})
		}
		return
	}

	if req.Password != req.PasswordConfirmation {
		slog.InfoContext(ctx, "someone tried to register with different passwords")
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: "passwords don't match"})
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
			slog.String("user_email", req.Email),
		)
		WriteResponse(w, http.StatusInternalServerError, models.Error{Message: err.Error()})
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
		switch err {
		case services.ErrUserNotFound, services.ErrInvalidPassword:
			slog.InfoContext(ctx,
				"someone tried to login with invalid credentials",
				slog.String("user_email", req.Email),
			)
			WriteResponse(w, http.StatusUnauthorized, models.Error{Message: "invalid credentials"})
		default:
			slog.ErrorContext(ctx,
				"error logging in user",
				slog.String("error", err.Error()),
				slog.String("user_email", req.Email),
			)
			WriteResponse(w, http.StatusInternalServerError, models.Error{Message: err.Error()})
		}
		return
	}

	var tokenString string
	if _, tokenString, err = validation.TokenAuth.Encode(claims); err != nil {
		slog.ErrorContext(ctx,
			"error generating signed token string",
			slog.String("error", err.Error()),
			slog.Any("claims", claims),
		)
		WriteResponse(w, http.StatusInternalServerError, models.Error{Message: "internal server error, try again later"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		HttpOnly: true,                              // JS cannot access it
		Secure:   os.Getenv(debugModeEnv) != "true", // HTTPS only (disable in local dev)
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   int((30 * time.Minute).Seconds()),
		Expires:  time.Now().Add(30 * time.Minute),
	})

	WriteResponse(w, http.StatusOK, nil)
}
