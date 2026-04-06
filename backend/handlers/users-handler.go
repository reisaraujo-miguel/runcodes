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
		msg := "Invalid sign up request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, models.Error{Message: msg})
		return
	}

	req.UserName = strings.TrimSpace(req.UserName)
	req.Email = strings.TrimSpace(req.Email)

	if err := validation.ValidateRequiredString(req.UserName, 100); err != nil {
		msg := "Invalid user name"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, models.Error{Message: err.Error()})
		return
	}

	if err := validation.ValidateEmailFormat(ctx, req.Email); err != nil {
		msg := "Invalid email"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, models.Error{Message: err.Error()})
		return
	}

	if emailExists, err := services.CheckEmailExistence(ctx, req.Email); err != nil {
		var msg string
		if emailExists {
			msg = "email already exists"
			WriteResponse(w, http.StatusConflict, msg, models.Error{Message: err.Error()})
		} else {
			msg = "database error validating email"
			WriteResponse(w, http.StatusInternalServerError, msg, models.Error{Message: err.Error()})
		}
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return
	}

	if req.Password != req.PasswordConfirmation {
		msg := "passwords don't match"
		slog.ErrorContext(ctx, msg)
		WriteResponse(w, http.StatusBadRequest, msg, models.Error{Message: msg})
		return
	}

	if err := validation.ValidatePassword(req.Password); err != nil {
		msg := "invalid password"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, models.Error{Message: err.Error()})
		return
	}

	if err := services.SignUp(ctx, &req); err != nil {
		msg := "error registering new user"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError, msg, models.Error{Message: err.Error()})
		return
	}

	WriteResponse(w, http.StatusCreated, "new user created", nil)
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
