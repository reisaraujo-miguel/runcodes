// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// tokenClaims retrieves the JWT claims or writes 401 and reports failure.
func tokenClaims(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	ctx := r.Context()
	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error retrieving claims from context",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return nil, false
	}
	return claims, true
}

/*
writeServiceError maps the services layer sentinel errors to HTTP responses.
*/
func writeServiceError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotOwner),
		errors.Is(err, services.ErrNotEnrolled):
		WriteResponse(w, http.StatusForbidden, models.Error{Message: err.Error()})
	case errors.Is(err, services.ErrOfferingNotFound),
		errors.Is(err, services.ErrExerciseNotFound),
		errors.Is(err, services.ErrTestCaseNotFound),
		errors.Is(err, services.ErrCompilationFileNotFound),
		errors.Is(err, services.ErrAttachedFileNotFound),
		errors.Is(err, services.ErrUserNotFound),
		errors.Is(err, services.ErrMemberNotFound),
		errors.Is(err, services.ErrFileTypeNotFound),
		errors.Is(err, services.ErrInvalidEnrollmentCode):
		WriteResponse(w, http.StatusNotFound, models.Error{Message: err.Error()})
	case errors.Is(err, services.ErrInvalidFileType),
		errors.Is(err, services.ErrInvalidTestCase),
		errors.Is(err, services.ErrInvalidExercise),
		errors.Is(err, services.ErrInvalidEmail),
		errors.Is(err, services.ErrInvalidMemberRole),
		errors.Is(err, services.ErrInvalidRole),
		errors.Is(err, services.ErrInvalidArchive),
		errors.Is(err, services.ErrInvalidTransfer),
		errors.Is(err, services.ErrNotPreviewing),
		errors.Is(err, services.ErrInvalidPreviewRole),
		errors.Is(err, services.ErrOwnerReferenceConflict),
		errors.Is(err, services.ErrOwnerReferenceEmpty),
		errors.Is(err, validation.ErrRequiredField),
		errors.Is(err, validation.ErrInputTooLong),
		errors.Is(err, validation.ErrParsingField):
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
	case errors.Is(err, services.ErrArchiveTooLarge):
		WriteResponse(w, http.StatusRequestEntityTooLarge,
			models.Error{Message: err.Error()},
		)
	case errors.Is(err, services.ErrJudgeUnavailable):
		WriteResponse(w, http.StatusServiceUnavailable,
			models.Error{Message: err.Error()},
		)
	case errors.Is(err, services.ErrNotAProfessor),
		errors.Is(err, services.ErrCannotPreview),
		errors.Is(err, services.ErrImportFailed):
		// The request is well formed; it is the account or archive content that
		// cannot be used, which the admin panel or a corrected archive can fix.
		WriteResponse(w, http.StatusUnprocessableEntity,
			models.Error{Message: err.Error()},
		)
	case errors.Is(err, services.ErrEmailExists),
		errors.Is(err, services.ErrOrgIDExists),
		errors.Is(err, services.ErrEnrollmentClosed),
		errors.Is(err, services.ErrEnrollmentBanned),
		errors.Is(err, services.ErrOfferingEnded),
		errors.Is(err, services.ErrOfferingOwned),
		errors.Is(err, services.ErrOwnerCannotUnenroll),
		errors.Is(err, services.ErrCannotBanOwner),
		errors.Is(err, services.ErrAlreadyMember),
		errors.Is(err, services.ErrSelfManagement),
		errors.Is(err, services.ErrSelfUnconfirm),
		errors.Is(err, services.ErrFileTypeExists),
		errors.Is(err, services.ErrFileTypeInUse):
		WriteResponse(w, http.StatusConflict, models.Error{Message: err.Error()})
	default:
		slog.ErrorContext(ctx, "unhandled service error",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
	}
}

/*
pathID parses an int64 URL parameter.
*/
func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, name), 10, 64)
}

/*
ListAllowedFileTypes handles GET /api/v1/allowed-file-types.
*/
func ListAllowedFileTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	types, err := services.ListAllowedFileTypes(ctx)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, types)
}

/*
parseCreateExerciseRequest decodes and validates the create-exercise body.
*/
func parseCreateExerciseRequest(body io.Reader) (*models.CreateExerciseRequest, error) {
	var req models.CreateExerciseRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}

	req.Title = strings.TrimSpace(req.Title)
	if err := validation.ValidateRequiredString(
		req.Title, services.MaxExerciseTitleLength,
	); err != nil {
		return nil, err
	}
	if req.Deadline == nil || req.OpenDate == nil {
		return nil, validation.ErrRequiredField
	}
	if req.Deadline.Before(*req.OpenDate) {
		return nil, services.ErrInvalidExercise
	}

	return &req, nil
}

/*
parseUpdateExerciseRequest decodes and validates the partial update body.
*/
func parseUpdateExerciseRequest(body io.Reader) (*models.UpdateExerciseRequest, error) {
	var req models.UpdateExerciseRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}

	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		req.Title = &trimmed
		if err := validation.ValidateRequiredString(
			trimmed, services.MaxExerciseTitleLength,
		); err != nil {
			return nil, err
		}
	}
	if req.Deadline != nil && req.OpenDate != nil && req.Deadline.Before(*req.OpenDate) {
		return nil, services.ErrInvalidExercise
	}

	return &req, nil
}

/*
isClientInputError reports whether err is a validation error (as opposed to a
malformed JSON body).
*/
func isClientInputError(err error) bool {
	return errors.Is(err, validation.ErrRequiredField) ||
		errors.Is(err, validation.ErrInputTooLong) ||
		errors.Is(err, services.ErrInvalidExercise)
}

/*
CreateExercise handles POST /api/v1/offerings/{offeringId}/exercises.
*/
func CreateExercise(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "offeringId")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	req, err := parseCreateExerciseRequest(r.Body)
	if err != nil {
		msg := "invalid exercise creation request"
		if isClientInputError(err) {
			msg = err.Error()
		}
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	exercise, err := services.CreateExercise(ctx, offeringID, req, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, exercise)
}

/*
ListOfferingExercises handles GET /api/v1/offerings/{offeringId}/exercises.
*/
func ListOfferingExercises(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "offeringId")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	exercises, err := services.ListOfferingExercises(ctx, offeringID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, exercises)
}

/*
GetExercise handles GET /api/v1/exercises/{id}.
*/
func GetExercise(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	exercise, err := services.GetExercise(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, exercise)
}

/*
UpdateExercise handles PUT /api/v1/exercises/{id}.
*/
func UpdateExercise(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	req, err := parseUpdateExerciseRequest(r.Body)
	if err != nil {
		msg := "invalid exercise update request"
		if isClientInputError(err) {
			msg = err.Error()
		}
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}

	exercise, err := services.UpdateExercise(ctx, exerciseID, req, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, exercise)
}

/*
DeleteExercise handles DELETE /api/v1/exercises/{id} (soft delete).
*/
func DeleteExercise(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	if err := services.DeleteExercise(ctx, exerciseID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
parseOptionalBool reads a "true"/"false" form field. Absent fields are nil.
*/
