// Package utils provides utility functions for JWT token generation and validation.
package utils

import (
	"os"

	"github.com/go-chi/jwtauth/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

var TokenAuth *jwtauth.JWTAuth

func init() {
	TokenAuth = jwtauth.New("HS256", jwtSecret, nil) // replace with secret key
}
