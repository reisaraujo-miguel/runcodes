package handlers

import (
	"encoding/json"
	"net/http"

	"runcodes/models"
)

const (
	InvalidPasswordErr = iota
	InvalidUserNameErr
	InvalidEmailErr
	PasswordsDontMatchErr
	EmailAlreadyExistsErr
	ServerErr
	InvalidRequestErr
)

/*
WriteResponse encodes a models.Response and writes using the provided http.ResponseWriter
*/
func WriteResponse(w http.ResponseWriter, httpStatus int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(httpStatus)
	response := models.Response{
		Message: msg,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}
