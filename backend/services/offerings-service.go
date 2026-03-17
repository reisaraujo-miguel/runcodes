// Package services provides business logic services for the application.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"runcodes/models"
	"runcodes/utils"
	"runcodes/validation"
)

/*
enrollmentCodeExists checks in the database if a given enrollment code ia already being used.
*/
func enrollmentCodeExists(code string) (bool, error) {
	var exists bool
	err := utils.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM offerings WHERE enrollment_code=$1)", code).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("Error scanning row: %s", err)
	}
	return exists, nil
}

/*
generateEnrollmentCode generates a random 4 characters alphanumeric enrollment code
*/
func generateEnrollmentCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxAttempts = 100

	for range maxAttempts {
		result := make([]byte, 4)

		for i := range result {
			result[i] = charset[rand.IntN(len(charset))]
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

/*
CreateOffering creates a new offering using the models.Offering model.
It either returns a nil offering and a http error status code or returns a valid offering and http.StatusCreated.

HTTP status is returned using the models.HTTPStatus struct.
*/
func CreateOffering(req *models.CreateOfferingRequest, ctx context.Context) (*models.Offering, models.HTTPStatus, error) {
	if err := validation.ValidateCreateOfferingRequest(req.Email, req.Name, req.EndDate); err != nil {
		msg := "Error validating offering creation"
		slog.ErrorContext(ctx, msg, slog.String("Error", err.Error()))
		return nil, models.HTTPStatus{StatusCode: http.StatusBadRequest, Msg: msg}, err
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	enrollmentCode, err := generateEnrollmentCode()
	if err != nil {
		msg := "Error generating enrollment code"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return nil, models.HTTPStatus{StatusCode: http.StatusInternalServerError, Msg: msg}, err
	}

	var newOfferingID string

	year := time.Now().Year()

	term := 1
	if time.Now().Year() > 6 {
		term = 2
	}

	err = utils.DB.QueryRow(
		`INSERT INTO offerings (course_id, year, term, classroom, end_date, enrollment_code) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		1, year, term, req.Name, req.EndDate, enrollmentCode,
	).Scan(&newOfferingID)
	if err != nil {
		msg := "Database error creating offering"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return nil, models.HTTPStatus{StatusCode: http.StatusInternalServerError, Msg: msg}, err
	}

	offering := &models.Offering{
		ID:             newOfferingID,
		Name:           req.Name,
		EndDate:        req.EndDate,
		EnrollmentCode: enrollmentCode,
	}

	return offering, models.HTTPStatus{StatusCode: http.StatusCreated, Msg: "Offering created"}, nil
}

/*
GetOfferings returns the fields "id", "name", and "end_date" for every available offering.
*/
func GetOfferings(ctx context.Context) ([]models.Offering, models.HTTPStatus, error) {
	rows, err := utils.DB.Query("SELECT id, name, end_date FROM offerings")
	if err != nil {
		msg := "Database error fetching offerings"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return nil, models.HTTPStatus{StatusCode: http.StatusInternalServerError, Msg: msg}, err
	}
	defer rows.Close()

	var offerings []models.Offering
	for rows.Next() {
		var offering models.Offering
		if err := rows.Scan(&offering.ID, &offering.Name, &offering.EndDate); err != nil {
			slog.ErrorContext(ctx, "Row scanning error", slog.String("error", err.Error()))
			continue
		}
		offerings = append(offerings, offering)
	}

	if err := rows.Err(); err != nil {
		msg := "Row iteration error"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return nil, models.HTTPStatus{StatusCode: http.StatusInternalServerError, Msg: msg}, err
	}

	return offerings, models.HTTPStatus{StatusCode: http.StatusOK, Msg: "offerings gathered"}, nil
}

/*
GetOfferingByID returns the "name" and "end_date" of a specific offering delimited by the offering id key
*/
func GetOfferingByID(id string, ctx context.Context) (*models.Offering, models.HTTPStatus, error) {
	var offering models.Offering

	err := utils.DB.QueryRow("SELECT id, name, end_date FROM offerings WHERE id = $1", id).
		Scan(&offering.ID, &offering.Name, &offering.EndDate)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			msg := fmt.Sprintf("Offering with ID %s not found", id)
			slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
			return nil, models.HTTPStatus{StatusCode: http.StatusNotFound, Msg: msg}, err
		default:
			msg := "Database error fetching offering"
			slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
			return nil, models.HTTPStatus{StatusCode: http.StatusInternalServerError, Msg: msg}, err
		}
	}

	return &offering, models.HTTPStatus{StatusCode: http.StatusFound, Msg: "offering found"}, nil
}
