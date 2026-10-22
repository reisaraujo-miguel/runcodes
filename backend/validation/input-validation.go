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
meet the criteria
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

/*
ValidateOptionalString validates a value that may be empty, such as a disclaimer
an admin chose to clear, and only bounds its length.
*/
func ValidateOptionalString(value string, maxSize int) error {
	if utf8.RuneCountInString(value) > maxSize {
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

	// bcrypt silently refuses inputs over 72 bytes (and GenerateFromPassword
	// returns an error), so reject them here as a validation error rather than
	// letting hashing fail as an internal error.
	if len(password) > 72 {
		return ErrPasswordTooLong
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

/*
NormalizeEmail canonicalises an address for storage and lookup. The domain and
local part are treated case-insensitively: a user who signed up as
Ada@Example.com must be able to sign in, reset a password or be looked up by an
administrator with ada@example.com. The database keeps a unique index on
lower(email) so two case variants cannot become separate accounts.
*/
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
