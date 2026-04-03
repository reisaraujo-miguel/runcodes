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
		WriteResponse(w, http.StatusBadRequest, msg, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	if err := validation.ValidateRequiredString(req.Name, 100); err != nil {
		msg := "invalid name"
		slog.ErrorContext(ctx, msg, slog.String("Error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "name", "error_msg": err.Error()})
		return
	}

	if date, err := validation.ValidateDate(ctx, req.EndDate); err != nil {
		msg := "invalid date"
		slog.ErrorContext(ctx, msg, slog.String("Error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "date", "error_msg": err.Error()})
		return
	} else if date.Before(time.Now()) {
		msg := "end date can't be before the creation date"
		slog.ErrorContext(ctx, "invalid date", slog.String("Error", msg))
		WriteResponse(w, http.StatusBadRequest, msg, map[string]string{"error_type": "date", "error_msg": "end before creation"})
		return
	}

	if err := services.CreateOffering(ctx, &req); err != nil {
		msg := "Failed to create offering"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError, msg, err.Error())
		return
	}

	WriteResponse(w, http.StatusCreated, "new offering created", nil)
}
