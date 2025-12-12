// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"runcodes/errors"
	"runcodes/models"
	"runcodes/services"
)

func CreateOffering(db *sql.DB) http.HandlerFunc {
	offeringService := services.NewOfferingService(db)

	return func(w http.ResponseWriter, r *http.Request) {
		var req models.CreateOfferingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteError(w, errors.ErrInvalidRequest)
			return
		}

		offering, token, err := offeringService.CreateOffering(&req)
		if err != nil {
			errors.HandleError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		response := models.Response{
			Success: true,
			Token:   token,
			Message: "Offering created successfully",
			Data: map[string]any{
				"offering":        offering,
				"enrollment_code": offering.ID,
			},
		}

		json.NewEncoder(w).Encode(response)
	}
}

func GetOfferings(db *sql.DB) http.HandlerFunc {
	offeringService := services.NewOfferingService(db)

	return func(w http.ResponseWriter, r *http.Request) {
		offerings, err := offeringService.GetOfferings()
		if err != nil {
			errors.HandleError(w, err)
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
}

func GetOfferingByID(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract offering ID from URL parameters
		// This would typically use gorilla/mux or similar router
		// For now, we'll implement this when the router supports it
		errors.WriteError(w, errors.NewAppError(http.StatusNotImplemented, "Not implemented", nil))
	}
}
