// Package models defines the data structures used in the application.
package models

type Response struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
