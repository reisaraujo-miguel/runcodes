package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"runcodes/models"
)

func loginUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(ctx, "Failed to decode login request", slog.String("error", err.Error()))
		return
	}
}
