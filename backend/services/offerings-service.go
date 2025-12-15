// Package services provides business logic services for the application.
package services

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	"runcodes/errors"
	"runcodes/models"
	"runcodes/utils"
	"runcodes/validation"
)

type OfferingService struct {
	db *sql.DB
}

func NewOfferingService(db *sql.DB) *OfferingService {
	return &OfferingService{db: db}
}

func (s *OfferingService) enrollmentCodeExists(code string) (bool, error) {
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM offerings WHERE enrollment_code=$1)", code).Scan(&exists)
	if err != nil {
		return false, errors.NewAppError(errors.ErrDatabase.Code, "Failed to check enrollment code", err)
	}
	return exists, nil
}

func (s *OfferingService) generateEnrollmentCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const maxAttempts = 100

	for range maxAttempts {
		result := make([]byte, 4)

		for i := range result {
			result[i] = charset[rand.Intn(len(charset))]
		}

		code := string(result)

		exists, err := s.enrollmentCodeExists(code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	return "", errors.NewAppError(errors.ErrInternalServer.Code, "Failed to generate unique enrollment code after maximum attempts", nil)
}

func (s *OfferingService) CreateOffering(req *models.CreateOfferingRequest) (*models.Offering, string, error) {
	if err := validation.ValidateCreateOfferingRequest(req.Email, req.Name, req.EndDate); err != nil {
		return nil, "", err
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.EndDate = strings.TrimSpace(req.EndDate)

	enrollmentCode, err := s.generateEnrollmentCode()
	if err != nil {
		return nil, "", err
	}

	//
	var newOfferingID string

	///////////////////////////////////////////////////////////////////////////////////////////
	// LEGACY: course_id, year, term are set to 0 because these fields are being deprecated. //
	// Will be removed when we migrate the database to a new version and schema.             //
	///////////////////////////////////////////////////////////////////////////////////////////
	err = s.db.QueryRow(
		`INSERT INTO offerings (course_id, year, term, classroom, end_date, enrollment_code) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		0, 0, 0, req.Name, req.EndDate, enrollmentCode,
	).Scan(&newOfferingID)
	if err != nil {
		errors.LogError("offerings-service.go", "CreateOffering", "Database error creating offering", err)
		return nil, "", errors.NewAppError(errors.ErrDatabase.Code, "Failed to create offering", err)
	}

	token, err := utils.GenerateToken(newOfferingID, req.Email)
	if err != nil {
		errors.LogError("offerings-service.go", "CreateOffering", "Token generation error", err)
		return nil, "", errors.NewAppError(errors.ErrTokenGeneration.Code, "Failed to generate token", err)
	}

	offering := &models.Offering{
		ID:             newOfferingID,
		Name:           req.Name,
		EndDate:        req.EndDate,
		EnrollmentCode: enrollmentCode,
	}

	return offering, token, nil
}

func (s *OfferingService) GetOfferings() ([]models.Offering, error) {
	rows, err := s.db.Query("SELECT id, name, end_date FROM offerings")
	if err != nil {
		errors.LogError("offerings-service.go", "GetOfferings", "Database error fetching offerings", err)
		return nil, errors.NewAppError(errors.ErrDatabase.Code, "Failed to retrieve offerings", err)
	}
	defer rows.Close()

	var offerings []models.Offering
	for rows.Next() {
		var offering models.Offering
		if err := rows.Scan(&offering.ID, &offering.Name, &offering.EndDate); err != nil {
			errors.LogError("offerings-service.go", "GetOfferings", "Row scanning error", err)
			continue
		}
		offerings = append(offerings, offering)
	}

	if err := rows.Err(); err != nil {
		errors.LogError("offerings-service.go", "GetOfferings", "Row iteration error", err)
		return nil, errors.NewAppError(errors.ErrDatabase.Code, "Failed to process offerings", err)
	}

	return offerings, nil
}

func (s *OfferingService) GetOfferingByID(id string) (*models.Offering, error) {
	var offering models.Offering

	err := s.db.QueryRow("SELECT id, name, end_date FROM offerings WHERE id = $1", id).
		Scan(&offering.ID, &offering.Name, &offering.EndDate)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewAppError(errors.ErrNotFound.Code, fmt.Sprintf("Offering with ID %s not found", id), nil)
		}

		errors.LogError("offerings-service.go", "GetOfferingByID", "Database error fetching offering", err)
		return nil, errors.NewAppError(errors.ErrDatabase.Code, "Failed to retrieve offering", err)
	}

	return &offering, nil
}
