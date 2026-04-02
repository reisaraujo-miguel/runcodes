// Package handlers defines the HTTP handlers for the application.
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

/*
CreateOffering handles new offering creations.
*/
func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	/* do later
	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		msg := "Failed to authenticate"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, false, msg, err.Error())
		return
	}
	*/

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Invalid offering creation request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	if err := validation.CreateOfferingValidation(&req, ctx); err != nil {
		msg := "error on request validation"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, msg, err.Error())
	}

	if err := services.CreateOffering(ctx, &req); err != nil {
		msg := "Failed to create offering"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError, msg, err.Error())
		return
	}

	WriteResponse(w, http.StatusCreated, "new offering created", nil)
}
