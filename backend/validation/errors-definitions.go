package validation

import "errors"

var (
	ErrRequiredField               = errors.New("field is required")
	ErrInputTooLong                = errors.New("input is too long")
	ErrParsingField                = errors.New("error parsing field")
	ErrInputTooShort               = errors.New("input is too short")
	ErrPasswordTooLong             = errors.New("password must be at most 72 bytes")
	ErrMustContainUppercase        = errors.New("password must contain at least one uppercase letter")
	ErrMustContainLowercase        = errors.New("password must contain at least one lowercase letter")
	ErrMustContainDigit            = errors.New("password must contain at least one digit")
	ErrMustContainSpecialCharacter = errors.New("password must contain at least one special character")
)
