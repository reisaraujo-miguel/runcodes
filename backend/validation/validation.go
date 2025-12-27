// Package validation provides input validation and sanitization utilities.
package validation

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return err
	}

	return nil
}

func ValidateName(name string, fieldName string) error {
	if name == "" {
		return errors.New("name is required")
	}

	if len(name) > 100 {
		return errors.New("name must be less than 100 characters")
	}

	// Check for potentially dangerous characters
	dangerousChars := regexp.MustCompile(`[<>{}]`)
	if dangerousChars.MatchString(name) {
		return errors.New("name contains invalid characters: '<', '>', '{' or '}'")
	}

	return nil
}

func ValidateDate(dateStr string, fieldName string) error {
	if dateStr == "" {
		return nil
	}

	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return err
	}

	return nil
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
