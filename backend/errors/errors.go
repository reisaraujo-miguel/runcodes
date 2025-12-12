// Package errors defines custom error types and error handling utilities for the application.
package errors

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// AppError represents an application error with HTTP status code and message
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error implements the AppError interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Common application errors
var (
	ErrInvalidRequest  = NewAppError(http.StatusBadRequest, "Invalid request", nil)
	ErrUnauthorized    = NewAppError(http.StatusUnauthorized, "Unauthorized", nil)
	ErrForbidden       = NewAppError(http.StatusForbidden, "Forbidden", nil)
	ErrNotFound        = NewAppError(http.StatusNotFound, "Resource not found", nil)
	ErrInternalServer  = NewAppError(http.StatusInternalServerError, "Internal server error", nil)
	ErrDatabase        = NewAppError(http.StatusInternalServerError, "Database error", nil)
	ErrValidation      = NewAppError(http.StatusBadRequest, "Validation failed", nil)
	ErrTokenGeneration = NewAppError(http.StatusInternalServerError, "Token generation failed", nil)
	ErrTokenValidation = NewAppError(http.StatusUnauthorized, "Token validation failed", nil)
)

func WriteError(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)

	response := map[string]any{
		"success": false,
		"message": err.Message,
		"error":   err.Error(),
	}

	if err.Err != nil {
		response["details"] = err.Err.Error()
	}

	json.NewEncoder(w).Encode(response)
}

// HandleError inspects the error type and writes an appropriate HTTP response
func HandleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*AppError); ok {
		WriteError(w, appErr)
		return
	}

	// Default to internal server error for unknown errors
	WriteError(w, NewAppError(http.StatusInternalServerError, "Internal server error", err))
}

// LogError logs the error with timestamp, file, and function context
func LogError(file string, function string, message string, err error) {
	log.Printf("Error in %s:%s - %s: %v\n", file, function, message, err)
}

func LogFatalError(file string, function string, message string, err error) {
	log.Fatalf("Fatal Error in %s:%s - %s: %v\n", file, function, message, err)
}
