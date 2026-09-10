// Package handlers defines the HTTP handlers for the application.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
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
		errors.Is(err, services.ErrAttachedFileNotFound):
		WriteResponse(w, http.StatusNotFound, models.Error{Message: err.Error()})
	case errors.Is(err, services.ErrInvalidFileType),
		errors.Is(err, services.ErrInvalidTestCase),
		errors.Is(err, services.ErrInvalidExercise),
		errors.Is(err, validation.ErrRequiredField),
		errors.Is(err, validation.ErrInputTooLong),
		errors.Is(err, validation.ErrParsingField):
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
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
func parseOptionalBool(form *multipart.Form, key string) (*bool, error) {
	values, ok := form.Value[key]
	if !ok || len(values) == 0 {
		return nil, nil
	}

	value, err := strconv.ParseBool(strings.TrimSpace(values[0]))
	if err != nil {
		return nil, services.ErrInvalidTestCase
	}
	return &value, nil
}

func parseOptionalString(form *multipart.Form, key string) *string {
	values, ok := form.Value[key]
	if !ok || len(values) == 0 {
		return nil
	}
	value := values[0]
	return &value
}

/*
parseOptionalInt reads a numeric form field, tolerating a decimal value (the
UI's seconds input uses a fractional step) by rounding to the nearest integer.
An empty value is treated as absent.
*/
func parseOptionalInt(form *multipart.Form, key string) (*int, error) {
	raw := parseOptionalString(form, key)
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}

	text := strings.TrimSpace(*raw)
	if value, err := strconv.Atoi(text); err == nil {
		return &value, nil
	}

	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, services.ErrInvalidTestCase
	}
	value := int(math.Round(parsed))
	return &value, nil
}

func parseOptionalInt64(form *multipart.Form, key string) (*int64, error) {
	raw := parseOptionalString(form, key)
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(*raw), 10, 64)
	if err != nil {
		return nil, services.ErrInvalidTestCase
	}
	return &value, nil
}

/*
readUploadedFile buffers a multipart file part in memory (bounded).
*/
func readUploadedFile(header *multipart.FileHeader) (*services.UploadedFile, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, services.MaxAuthoringUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > services.MaxAuthoringUploadBytes {
		return nil, services.ErrInvalidTestCase
	}

	return &services.UploadedFile{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Data:        data,
	}, nil
}

func firstUploadedFile(
	form *multipart.Form, key string,
) (*services.UploadedFile, error) {
	headers, ok := form.File[key]
	if !ok || len(headers) == 0 {
		return nil, nil
	}
	return readUploadedFile(headers[0])
}

/*
parseTestCaseInput maps a parsed multipart form onto a TestCaseInput. It only
decodes values; cross-field validation happens in the service layer.
*/
func parseTestCaseInput(form *multipart.Form, partial bool) (*services.TestCaseInput, error) {
	_ = partial

	in := &services.TestCaseInput{
		InputType:          parseOptionalString(form, "input_type"),
		ExpectedOutputType: parseOptionalString(form, "expected_output_type"),
		Input:              parseOptionalString(form, "input"),
		ExpectedOutput:     parseOptionalString(form, "expected_output"),
	}

	var err error
	if in.ShowInput, err = parseOptionalBool(form, "show_input"); err != nil {
		return nil, err
	}
	if in.ShowExpectedOutput, err = parseOptionalBool(form, "show_expected_output"); err != nil {
		return nil, err
	}
	if in.ShowUserOutput, err = parseOptionalBool(form, "show_user_output"); err != nil {
		return nil, err
	}

	if in.CPUTimeLimitSeconds, err = parseOptionalInt(form, "cpu_time_limit_seconds"); err != nil {
		return nil, err
	}
	if in.MemUsageLimitBytes, err = parseOptionalInt64(form, "mem_usage_limit_bytes"); err != nil {
		return nil, err
	}
	if in.StackLimitBytes, err = parseOptionalInt64(form, "stack_limit_bytes"); err != nil {
		return nil, err
	}
	if in.FileSizeLimitBytes, err = parseOptionalInt64(form, "file_size_limit_bytes"); err != nil {
		return nil, err
	}

	if in.InputFile, err = firstUploadedFile(form, "input_file"); err != nil {
		return nil, err
	}
	if in.ExpectedOutputFile, err = firstUploadedFile(form, "expected_output_file"); err != nil {
		return nil, err
	}

	if headers, ok := form.File["files"]; ok && len(headers) > 0 {
		in.FilesProvided = true
		in.Files = make([]services.UploadedFile, 0, len(headers))
		for _, header := range headers {
			uploaded, err := readUploadedFile(header)
			if err != nil {
				return nil, err
			}
			in.Files = append(in.Files, *uploaded)
		}
	}

	return in, nil
}

/*
parseTestCaseRequest parses the multipart body of a test-case create/update.
*/
func parseTestCaseRequest(w http.ResponseWriter, r *http.Request, partial bool) (*services.TestCaseInput, bool) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxAuthoringUploadBytes)+(1<<20),
	)

	if err := r.ParseMultipartForm(services.MaxAuthoringUploadBytes); err != nil {
		msg := "invalid test case payload"
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			msg = "upload is too large"
			WriteResponse(w, http.StatusRequestEntityTooLarge, models.Error{Message: msg})
			return nil, false
		}
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return nil, false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	in, err := parseTestCaseInput(r.MultipartForm, partial)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: err.Error()})
		return nil, false
	}

	return in, true
}

/*
ListTestCases handles GET /api/v1/exercises/{id}/test-cases.
*/
func ListTestCases(w http.ResponseWriter, r *http.Request) {
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

	cases, err := services.ListTestCases(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, cases)
}

/*
CreateTestCase handles POST /api/v1/exercises/{id}/test-cases.
*/
func CreateTestCase(w http.ResponseWriter, r *http.Request) {
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

	in, ok := parseTestCaseRequest(w, r, false)
	if !ok {
		return
	}

	testCase, err := services.CreateTestCase(ctx, exerciseID, in, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, testCase)
}

/*
UpdateTestCase handles PUT /api/v1/exercises/{id}/test-cases/{caseId}.
*/
func UpdateTestCase(w http.ResponseWriter, r *http.Request) {
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
	caseID, err := pathID(r, "caseId")
	if err != nil || caseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid test case id"},
		)
		return
	}

	in, ok := parseTestCaseRequest(w, r, true)
	if !ok {
		return
	}

	testCase, err := services.UpdateTestCase(ctx, exerciseID, caseID, in, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, testCase)
}

/*
DeleteTestCase handles DELETE /api/v1/exercises/{id}/test-cases/{caseId}.
*/
func DeleteTestCase(w http.ResponseWriter, r *http.Request) {
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
	caseID, err := pathID(r, "caseId")
	if err != nil || caseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid test case id"},
		)
		return
	}

	if err := services.DeleteTestCase(ctx, exerciseID, caseID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
parseSingleFileRequest parses a multipart body with a single "file" field.
*/
func parseSingleFileRequest(w http.ResponseWriter, r *http.Request) (*services.UploadedFile, bool) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxAuthoringUploadBytes)+(1<<20),
	)

	if err := r.ParseMultipartForm(services.MaxAuthoringUploadBytes); err != nil {
		msg := "invalid file upload payload"
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteResponse(w, http.StatusRequestEntityTooLarge,
				models.Error{Message: "upload is too large"},
			)
			return nil, false
		}
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return nil, false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, err := firstUploadedFile(r.MultipartForm, "file")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return nil, false
	}
	if file == nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "file is required"},
		)
		return nil, false
	}

	return file, true
}

/*
ListCompilationFiles handles GET /api/v1/exercises/{id}/compilation-files.
*/
func ListCompilationFiles(w http.ResponseWriter, r *http.Request) {
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

	files, err := services.ListCompilationFiles(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, files)
}

/*
CreateCompilationFile handles POST /api/v1/exercises/{id}/compilation-files.
*/
func CreateCompilationFile(w http.ResponseWriter, r *http.Request) {
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

	file, ok := parseSingleFileRequest(w, r)
	if !ok {
		return
	}

	compilationFile, err := services.CreateCompilationFile(ctx, exerciseID, *file, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, compilationFile)
}

/*
GetCompilationFile handles GET
/api/v1/exercises/{id}/compilation-files/{fileId}.
*/
func GetCompilationFile(w http.ResponseWriter, r *http.Request) {
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
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	file, err := services.GetCompilationFile(ctx, exerciseID, fileID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, file)
}

/*
DeleteCompilationFile handles DELETE
/api/v1/exercises/{id}/compilation-files/{fileId}.
*/
func DeleteCompilationFile(w http.ResponseWriter, r *http.Request) {
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
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	if err := services.DeleteCompilationFile(ctx, exerciseID, fileID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
ListAttachedFiles handles GET /api/v1/exercises/{id}/attached-files.
*/
func ListAttachedFiles(w http.ResponseWriter, r *http.Request) {
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

	files, err := services.ListAttachedFiles(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, files)
}

/*
CreateAttachedFile handles POST /api/v1/exercises/{id}/attached-files.
*/
func CreateAttachedFile(w http.ResponseWriter, r *http.Request) {
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

	file, ok := parseSingleFileRequest(w, r)
	if !ok {
		return
	}

	attached, err := services.CreateAttachedFile(ctx, exerciseID, *file, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, attached)
}

/*
DeleteAttachedFile handles DELETE
/api/v1/exercises/{id}/attached-files/{fileId}.
*/
func DeleteAttachedFile(w http.ResponseWriter, r *http.Request) {
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
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	if err := services.DeleteAttachedFile(ctx, exerciseID, fileID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
