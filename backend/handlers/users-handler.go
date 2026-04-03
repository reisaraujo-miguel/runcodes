package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"runcodes/models"
	"runcodes/services"
	"runcodes/validation"
)

func SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.SignUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid sign in request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, nil)
		return
	}

	req.UserName = strings.TrimSpace(req.UserName)
	req.Email = strings.TrimSpace(req.Email)

	if err := validation.ValidateRequiredString(req.UserName, 100); err != nil {
		msg := "Invalid user name"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "user_name", "error_msg": err.Error()})
		return
	}

	if emailExists, err := validation.ValidateEmail(ctx, req.Email); err != nil {
		msg := "Invalid email"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		if emailExists {
			WriteResponse(w, http.StatusConflict, msg, map[string]string{"error_type": "email", "error_msg": err.Error()})
		} else {
			WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "email", "error_msg": err.Error()})
		}
		return
	}

	if req.Password != req.PasswordConfirmation {
		msg := "passwords don't match"
		slog.ErrorContext(ctx, msg)
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "password", "error_msg": msg})
		return
	}

	if err := validation.ValidatePassword(req.Password); err != nil {
		msg := "invalid password"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "password", "error_msg": err.Error()})
		return
	}

	if err := services.SignIn(ctx, &req); err != nil {
		msg := "error registering new user"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError, msg, nil)
		return
	}

	WriteResponse(w, http.StatusOK, "new user created", nil)
}

func LogIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	var req models.LogInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid login request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, nil)
		return
	}
}
