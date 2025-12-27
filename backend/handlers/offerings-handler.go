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

func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Logger.ErrorContext(ctx, "Failed to decode offering creation request", slog.String("error", err.Error()))
		return
	}

	offering := services.CreateOffering(&req, ctx)
	if offering == nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	response := models.Response{
		Success: true,
		Message: "Offering created successfully",
		Data: map[string]any{
			"offering":        offering,
			"enrollment_code": offering.ID,
		},
	}

	json.NewEncoder(w).Encode(response)
}

func GetOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offerings := services.GetOfferings(ctx)
	if offerings == nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := models.Response{
		Success: true,
		Message: "Offerings retrieved successfully",
		Data:    offerings,
	}

	json.NewEncoder(w).Encode(response)
}

func GetOfferingByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Extract offering ID from URL parameters
	// This would typically use gorilla/mux or similar router
	// For now, we'll implement this when the router supports it
	utils.Logger.ErrorContext(ctx, "GetOfferingByID not implemented")
}
