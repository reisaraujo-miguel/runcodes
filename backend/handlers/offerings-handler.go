// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"runcodes/models"
	"runcodes/services"
	"runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
)

/*
CreateOffering handles new offering creations.
*/
func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid offering creation request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	var claims map[string]any
	var err error
	if _, claims, err = jwtauth.FromContext(ctx); err != nil {
		slog.ErrorContext(ctx,
			"error retrieving claims from context",
			slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	if err := validation.ValidateRequiredString(req.Name, 100); err != nil {
		slog.InfoContext(ctx,
			"user tried to create an offering with an invalid name",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	}

	if date, err := validation.ValidateDate(ctx, req.EndDate); err != nil {
		slog.InfoContext(ctx,
			"user tried to create and offering with an invalid end date",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return
	} else if date.Before(time.Now()) {
		slog.InfoContext(ctx,
			"user tried to create an offering with an invalid end date",
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "end date cannot be in the past"},
		)
		return
	}

	if err := services.CreateOffering(ctx, &req, claims); err != nil {
		slog.ErrorContext(ctx,
			"Failed to create offering",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusCreated, nil)
}
