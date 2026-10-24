package handlers

import (
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
)

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
