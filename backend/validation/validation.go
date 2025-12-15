// Package validation provides input validation and sanitization utilities.
package validation

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"runcodes/errors"
)

func ValidateEmail(email string) error {
	if email == "" {
		return errors.NewAppError(errors.ErrValidation.Code, "Email is required", nil)
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return errors.NewAppError(errors.ErrValidation.Code, "Invalid email format", err)
	}

	return nil
}

func ValidateName(name string, fieldName string) error {
	if name == "" {
		return errors.NewAppError(errors.ErrValidation.Code, fieldName+" is required", nil)
	}

	if len(name) > 100 {
		return errors.NewAppError(errors.ErrValidation.Code, fieldName+" must be less than 100 characters", nil)
	}

	// Check for potentially dangerous characters
	dangerousChars := regexp.MustCompile(`[<>{}]`)
	if dangerousChars.MatchString(name) {
		return errors.NewAppError(errors.ErrValidation.Code, fieldName+" contains invalid characters: '<', '>', '{' or '}'", nil)
	}

	return nil
}

func ValidateDate(dateStr string, fieldName string) error {
	if dateStr == "" {
		return nil
	}

	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return errors.NewAppError(errors.ErrValidation.Code, fieldName+" must be in YYYY-MM-DD format", err)
	}

	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return errors.NewAppError(errors.ErrValidation.Code, "Password is required", nil)
	}

	if len(password) < 8 {
		return errors.NewAppError(errors.ErrValidation.Code, "Password must be at least 8 characters long", nil)
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
		return errors.NewAppError(errors.ErrValidation.Code, "Password must contain at least one uppercase letter", nil)
	}

	if !hasLower {
		return errors.NewAppError(errors.ErrValidation.Code, "Password must contain at least one lowercase letter", nil)
	}

	if !hasDigit {
		return errors.NewAppError(errors.ErrValidation.Code, "Password must contain at least one digit", nil)
	}

	if !hasSpecial {
		return errors.NewAppError(errors.ErrValidation.Code, "Password must contain at least one special character", nil)
	}

	return nil
}

func ValidateCreateOfferingRequest(email, name, endDate string) error {
	if err := ValidateEmail(email); err != nil {
		return err
	}

	if err := ValidateName(name, "Offering name"); err != nil {
		return err
	}

	if err := ValidateDate(endDate, "End date"); err != nil {
		return err
	}

	return nil
}
