// Package services provides business logic services for the application.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"

	"runcodes/models"

	"github.com/go-chi/jwtauth/v5"
)

/*
CreateOffering creates a new offering on the platform.
*/
func CreateOffering(ctx context.Context, req *models.CreateOfferingRequest) error {
	var claims map[string]any
	var err error
	if _, claims, err = jwtauth.FromContext(ctx); err != nil {
		msg := "failed to authenticate"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	OwnerID := claims["user_id"]

	var enrollmentCode string
	if enrollmentCode, err = generateEnrollmentCode(ctx); err != nil {
		msg := "error generating enrollment code"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	var tx *sql.Tx
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
		msg := "error initializing the transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO offerings (name, owner_id, end_date, enrollment_code, description) VALUES ($1, $2, $3, $4, $5)",
		req.Name, OwnerID, req.EndDate, enrollmentCode, req.Description,
	); err != nil {
		msg := "database error creating offering"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	if err := tx.Commit(); err != nil {
		msg := "error during database transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	return nil
}

/*
generateEnrollmentCode generates a random 4 characters alphanumeric enrollment code
*/
func generateEnrollmentCode(ctx context.Context) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxAttempts = 100

	for range maxAttempts {
		result := make([]byte, 4)

		for i := range result {
			result[i] = charset[rand.IntN(len(charset))]
		}

		code := string(result)

		if exists, err := enrollmentCodeExists(ctx, code); err != nil {
			msg := "error while checking if a new enrollment code is valid"
			slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
			return "", errors.New(msg)
		} else if !exists {
			return code, nil
		}
	}

	msg := "failed to generate unique enrollment code after maximum attempts"
	slog.ErrorContext(ctx, msg)

	return "", errors.New(msg)
}

/*
enrollmentCodeExists checks in the database if a given enrollment code is already being used.
It *can* return an error if the database check fails
*/
func enrollmentCodeExists(ctx context.Context, code string) (bool, error) {
	var tx *sql.Tx
	var err error
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
		msg := "error initializing the transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return false, errors.New(msg)
	}

	defer tx.Rollback()

	var id int
	rows := DB.QueryRowContext(ctx, "SELECT id FROM offerings WHERE enrollment_code = $1", code).Scan(&id)

	if err := tx.Commit(); err != nil {
		msg := "error during database transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return false, errors.New(msg)
	}

	if rows == sql.ErrNoRows {
		return false, nil // enrollment code does not exists
	} else if rows != nil {
		msg := "error querying enrolment code"
		slog.ErrorContext(ctx, msg, slog.String("error", rows.Error()))
		return false, errors.New(msg)
	}

	return true, nil // enrollment code does exists
}
