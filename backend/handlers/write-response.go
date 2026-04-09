package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

/*
WriteResponse encodes a http response and writes it
*/
func WriteResponse(w http.ResponseWriter, httpStatus int, data any) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
