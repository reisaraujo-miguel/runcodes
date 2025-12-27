// Package services provides business logic services for the application.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"

	"runcodes/models"
	"runcodes/utils"
	"runcodes/validation"
)

func enrollmentCodeExists(code string) (bool, error) {
	var exists bool
	err := utils.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM offerings WHERE enrollment_code=$1)", code).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func generateEnrollmentCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxAttempts = 100

	for range maxAttempts {
		result := make([]byte, 4)

		for i := range result {
			result[i] = charset[rand.Intn(len(charset))]
		}

		code := string(result)

		exists, err := enrollmentCodeExists(code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	err := errors.New("failed to generate unique enrollment code after maximum attempts")

	return "", err
}

func CreateOffering(req *models.CreateOfferingRequest, ctx context.Context) *models.Offering {
	if err := validation.ValidateCreateOfferingRequest(req.Email, req.Name, req.EndDate); err != nil {
		return nil
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	enrollmentCode, err := generateEnrollmentCode()
	if err != nil {
		return nil
	}

	var newOfferingID string

	///////////////////////////////////////////////////////////////////////////////////////////
	// LEGACY: course_id, year, term are set to 0 because these fields are being deprecated. //
	// Will be removed when we migrate the database to a new version and schema.             //
	///////////////////////////////////////////////////////////////////////////////////////////
	err = utils.DB.QueryRow(
		`INSERT INTO offerings (course_id, year, term, classroom, end_date, enrollment_code) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		0, 0, 0, req.Name, req.EndDate, enrollmentCode,
	).Scan(&newOfferingID)
	if err != nil {
		utils.Logger.ErrorContext(ctx, "Database error creating offering", slog.String("error", err.Error()))
		return nil
	}

	offering := &models.Offering{
		ID:             newOfferingID,
		Name:           req.Name,
		EndDate:        req.EndDate,
		EnrollmentCode: enrollmentCode,
	}

	return offering
}

func GetOfferings(ctx context.Context) []models.Offering {
	rows, err := utils.DB.Query("SELECT id, name, end_date FROM offerings")
	if err != nil {
		utils.Logger.ErrorContext(ctx, "Database error fetching offerings", slog.String("error", err.Error()))
		return nil
	}
	defer rows.Close()

	var offerings []models.Offering
	for rows.Next() {
		var offering models.Offering
		if err := rows.Scan(&offering.ID, &offering.Name, &offering.EndDate); err != nil {
			utils.Logger.ErrorContext(ctx, "Row scanning error", slog.String("error", err.Error()))
			continue
		}
		offerings = append(offerings, offering)
	}

	if err := rows.Err(); err != nil {
		utils.Logger.ErrorContext(ctx, "Row iteration error", slog.String("error", err.Error()))
		return nil
	}

	return offerings
}

func GetOfferingByID(id string, ctx context.Context) *models.Offering {
	var offering models.Offering

	err := utils.DB.QueryRow("SELECT id, name, end_date FROM offerings WHERE id = $1", id).
		Scan(&offering.ID, &offering.Name, &offering.EndDate)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.Logger.ErrorContext(ctx, fmt.Sprintf("Offering with ID %s not found", id), slog.String("error", err.Error()))
			return nil
		}

		utils.Logger.ErrorContext(ctx, "Database error fetching offering", slog.String("error", err.Error()))
		return nil
	}

	return &offering
}
