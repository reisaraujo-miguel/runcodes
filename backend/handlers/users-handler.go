package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"runcodes/models"
	"runcodes/utils"
)

func loginUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Logger.ErrorContext(ctx, "Failed to decode login request", slog.String("error", err.Error()))
		return
	}
}
