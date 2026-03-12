// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"runcodes/models"
	"runcodes/services"
	"runcodes/utils"
)

/*
CreateOffering handles new offering creations.
*/
func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := "Failed to decode offering creation request"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		utils.WriteResponse(w, http.StatusBadRequest, false, msg, nil)
		return
	}

	offering, httpStatus, err := services.CreateOffering(&req, ctx)
	if err != nil {
		msg := "Failed to create offering"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		utils.WriteResponse(w, httpStatus.StatusCode, false, msg, httpStatus.Msg)
		return
	}

	utils.WriteResponse(w, httpStatus.StatusCode, true, httpStatus.Msg, offering)
}

/*
GetOfferings handles querying for all existing offerings.
*/
func GetOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offerings, httpStatus, err := services.GetOfferings(ctx)
	if err != nil {
		msg := "Failed to retrieve offerings"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		utils.WriteResponse(w, httpStatus.StatusCode, false, msg, httpStatus.Msg)
		return
	}
		return
	}

	msg := "Offerings retrieved successfully"
	utils.WriteResponse(w, httpStatus.StatusCode, true, msg, offerings)
}

/*
GetOfferingByID handles querying for a specific offering by its id.
*/
func GetOfferingByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract offering ID from URL parameters
	// This would typically use gorilla/mux or similar router
	// For now, we'll implement this when the router supports it
	slog.ErrorContext(ctx, "GetOfferingByID not implemented")
}
