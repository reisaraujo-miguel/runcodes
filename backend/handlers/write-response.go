package handlers

import (
	"encoding/json"
	"net/http"
)

/*
WriteResponse encodes a models.Response and writes using the provided http.ResponseWriter
*/
func WriteResponse(w http.ResponseWriter, httpStatus int, data any) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(data)
}
