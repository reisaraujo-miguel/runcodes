package services

import "errors"

var (
	ErrEmailExists        = errors.New("email is already in use")
	ErrServer             = errors.New("internal server error, try again later")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
