// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"runcodes/models"
	"runcodes/services"
)

/*
CreateOffering handles new offering creations.
*/
func CreateOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateOfferingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(ctx, "Failed to decode offering creation request", slog.String("error", err.Error()))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		response := models.Response{
			Success: false,
			Message: "Failed to decode offering creation request",
			Data: map[string]any{
				"error": err.Error(),
			},
		}

		return
	}

	offering, httpStatus := services.CreateOffering(&req, ctx)

	if offering == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus.StatusCode)
		response := models.Response{
			Success: false,
			Message: "Failed to decode offering creation request",
			Data: map[string]any{
				"error": httpStatus.Msg,
			},
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus.StatusCode)

	response := models.Response{
		Success: true,
		Message: httpStatus.Msg,
		Data: map[string]any{
			"offering":        offering,
			"enrollment_code": offering.ID,
		},
	}

	json.NewEncoder(w).Encode(response)
}

/*
GetOfferings handles querying for all existing offerings.
*/
func GetOfferings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offerings, httpStatus := services.GetOfferings(ctx)
	if offerings == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus.StatusCode)
		response := models.Response{
			Success: false,
			Message: "Failed to decode offering creation request",
			Data: map[string]any{
				"error": httpStatus.Msg,
			},
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus.StatusCode)
	response := models.Response{
		Success: true,
		Message: "Offerings retrieved successfully",
		Data:    offerings,
	}

	json.NewEncoder(w).Encode(response)
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
