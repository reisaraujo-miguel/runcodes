// Package validation provides input validation, sanitization utilities and JWT validation.
package validation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"runcodes/services"
)

func ValidateEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, errors.New("email is required")
	}

	if _, err := mail.ParseAddress(email); err != nil {
		msg := "error parsing email address"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return false, errors.New(msg)
	}

	var id int
	err := services.DB.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1`,
		email,
	).Scan(&id)

	if err == sql.ErrNoRows {
		// email does not exists, so it is okay to use it
		return false, nil
	} else if err != nil {
		msg := "database error validating email"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return false, errors.New(msg)
	}

	return true, errors.New("email already in use")
}

/*
ValidateRequiredString validates if the string exists (is not an empty string) and is not
bigger than the allowed max_size, returning an error if it does not meets the criteria
*/
func ValidateRequiredString(name string, maxSize int) error {
	if name == "" {
		return errors.New("input is required")
	}

	if len(name) > maxSize {
		return fmt.Errorf("input must be smaller than %d characters", maxSize)
	}

	return nil
}

func ValidateDate(ctx context.Context, dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, errors.New("date is required")
	}

	var date time.Time
	var err error
	if date, err = time.Parse("2006-01-02", dateStr); err != nil {
		msg := "error parsing date string"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return nil, errors.New(msg)
	}

	return &date, nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("password is required")
	}

	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
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
		return errors.New("password must contain at least one uppercase letter")
	}

	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}

	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}

	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}
