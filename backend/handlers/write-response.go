package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

/*
WriteResponse encodes a http response and writes it.

Statuses that forbid a body (204, 304) are written without one: encoding into
them returns http.ErrBodyNotAllowed, which would otherwise log an error on every
successful delete.
*/
func WriteResponse(w http.ResponseWriter, httpStatus int, data any) {
	if httpStatus == http.StatusNoContent || httpStatus == http.StatusNotModified {
		w.WriteHeader(httpStatus)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
