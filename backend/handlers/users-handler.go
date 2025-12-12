package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"runcodes/errors"
	"runcodes/models"
)

func loginUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req models.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteError(w, errors.ErrInvalidRequest)
			return
		}
	}
}
