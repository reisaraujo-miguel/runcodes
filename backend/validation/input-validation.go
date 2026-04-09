// Package validation provides input validation, sanitization utilities
// and JWT validation.
package validation

import (
	"context"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

func ValidateEmailFormat(ctx context.Context, email string) error {
	if email == "" {
		return ErrRequiredField
	}

	if _, err := mail.ParseAddress(email); err != nil {
		slog.ErrorContext(ctx,
			"error parsing email address",
			slog.String("error", err.Error()),
		)
		return ErrParsingField
	}

	return nil
}

/*
ValidateRequiredString validates if the string exists (is not an empty string)
and is not bigger than the allowed max_size, returning an error if it does not
meets the criteria
*/
func ValidateRequiredString(name string, maxSize int) error {
	if name == "" {
		return ErrRequiredField
	}

	if utf8.RuneCountInString(name) > maxSize {
		return ErrInputTooLong
	}

	return nil
}

func ValidateDate(ctx context.Context, dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, ErrRequiredField
	}

	var date time.Time
	var err error
	if date, err = time.Parse(time.RFC3339Nano, dateStr); err != nil {
		slog.ErrorContext(ctx,
			"error parsing date",
			slog.String("error", err.Error()),
		)
		return nil, ErrParsingField
	}

	return &date, nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return ErrRequiredField
	}

	if len(password) < 8 {
		return ErrInputTooShort
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasDigit   = false
		hasSpecial = false
	)

	specialChars := "!@#$%^&*()_+-=[]{};':\"\\|,.<>/?~`"

	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case strings.ContainsRune(specialChars, ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return ErrMustContainUppercase
	}

	if !hasLower {
		return ErrMustContainLowercase
	}

	if !hasDigit {
		return ErrMustContainDigit
	}

	if !hasSpecial {
		return ErrMustContainSpecialCharacter
	}

	return nil
}
