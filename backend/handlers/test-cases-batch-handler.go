package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
batchArchiveFromRequest reads the `archive` part of a batch test-case upload
with the larger limit the feature allows.
*/
func batchArchiveFromRequest(
	w http.ResponseWriter, r *http.Request,
) ([]byte, bool) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxBatchArchiveBytes)+(1<<20),
	)

	if err := r.ParseMultipartForm(services.MaxBatchArchiveBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteResponse(w, http.StatusRequestEntityTooLarge,
				models.Error{Message: services.ErrArchiveTooLarge.Error()},
			)
			return nil, false
		}
		slog.ErrorContext(ctx, "invalid batch archive payload",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid batch archive payload"},
		)
		return nil, false
	}

	headers, ok := r.MultipartForm.File["archive"]
	if !ok || len(headers) == 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "archive is required"},
		)
		return nil, false
	}

	file, err := headers[0].Open()
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "could not read the archive"},
		)
		return nil, false
	}
	defer file.Close()

	data, err := io.ReadAll(
		io.LimitReader(file, services.MaxBatchArchiveBytes+1),
	)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "could not read the archive"},
		)
		return nil, false
	}
	if len(data) > services.MaxBatchArchiveBytes {
		WriteResponse(w, http.StatusRequestEntityTooLarge,
			models.Error{Message: services.ErrArchiveTooLarge.Error()},
		)
		return nil, false
	}

	return data, true
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func intOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func int64OrDefault(value *int64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return *value
}

/*
CreateTestCasesBatch handles POST /api/v1/exercises/{id}/test-cases/batch. The
multipart body carries the archive and the configuration shared by every case in
it. A malformed archive is answered with the list of folders that could not be
parsed, so the professor can fix all of them in one pass.
*/
func CreateTestCasesBatch(w http.ResponseWriter, r *http.Request) {
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

	archive, ok := batchArchiveFromRequest(w, r)
	if !ok {
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	form := r.MultipartForm

	inputType := strings.TrimSpace(formValue(form.Value, "input_type"))
	if inputType == "" {
		inputType = services.IOTypeText
	}
	outputType := strings.TrimSpace(formValue(form.Value, "expected_output_type"))
	if outputType == "" {
		outputType = services.IOTypeText
	}

	showInput, err := parseOptionalBool(form, "show_input")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}
	showOutput, err := parseOptionalBool(form, "show_expected_output")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}
	showUserOutput, err := parseOptionalBool(form, "show_user_output")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}

	cpu, err := parseOptionalInt(form, "cpu_time_limit_seconds")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}
	mem, err := parseOptionalInt64(form, "mem_usage_limit_bytes")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}
	stack, err := parseOptionalInt64(form, "stack_limit_bytes")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}
	fileSize, err := parseOptionalInt64(form, "file_size_limit_bytes")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return
	}

	config := services.BatchTestCaseConfig{
		InputType:           inputType,
		ExpectedOutputType:  outputType,
		ShowInput:           boolOrDefault(showInput, true),
		ShowExpectedOutput:  boolOrDefault(showOutput, true),
		ShowUserOutput:      boolOrDefault(showUserOutput, true),
		CPUTimeLimitSeconds: intOrDefault(cpu, 0),
		MemUsageLimitBytes:  int64OrDefault(mem, 0),
		StackLimitBytes:     int64OrDefault(stack, 0),
		FileSizeLimitBytes:  int64OrDefault(fileSize, 0),
	}

	result, err := services.CreateTestCasesBatch(
		ctx, exerciseID, archive, config, claims,
	)
	if err != nil {
		var batchErr *services.BatchImportError
		if errors.As(err, &batchErr) {
			WriteResponse(w, http.StatusUnprocessableEntity,
				models.BatchTestCaseErrorResponse{
					Message:        batchErr.Error(),
					InvalidFolders: batchErr.Issues,
				},
			)
			return
		}
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, result)
}

/*
formValue reads the first value of an optional form field.
*/
func formValue(values map[string][]string, key string) string {
	if list, ok := values[key]; ok && len(list) > 0 {
		return list[0]
	}
	return ""
}

/*
ExportExerciseTestCases handles GET
/api/v1/exercises/{id}/test-cases/export. It returns a zip in the same layout
the batch importer accepts, so cases can be moved between exercises.
*/
func ExportExerciseTestCases(w http.ResponseWriter, r *http.Request) {
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

	filename, archive, err := services.ExportExerciseTestCasesArchive(
		ctx, exerciseID, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+filename+`"`,
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := newDeadlineWriter(w).Write(archive); err != nil {
		slog.DebugContext(ctx, "client stopped the test cases download",
			slog.String("error", err.Error()),
		)
	}
}
