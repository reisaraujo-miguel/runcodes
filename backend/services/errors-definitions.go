package services

import "errors"

var (
	ErrEmailExists     = errors.New("email is already registered")
	ErrServer          = errors.New("internal server error, try again later")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
)
