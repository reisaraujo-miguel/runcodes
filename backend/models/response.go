// Package models defines the data structures used in the application.
package models

type Response struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Error struct {
	Message string `json:"error_msg"`
}
