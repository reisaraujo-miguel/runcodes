package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"

	"github.com/lib/pq"
)

// MaxAuthoringUploadBytes is the size cap for a single multipart authoring
// request (test case or file upload).
const MaxAuthoringUploadBytes = 32 << 20

// Input/output types accepted by `input_output_type_t`.
const (
	IOTypeText = "text"
	IOTypeFile = "file"
)

/*
UploadedFile is an in-memory file part of a multipart request. Authoring
requests are small, so buffering them keeps the service layer straightforward.
*/
type UploadedFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

/*
TestCaseInput is the parsed (but not yet validated) body of a test-case create
or update request. Pointer fields distinguish "absent" from a zero value, which
is what makes partial updates possible.
*/
type TestCaseInput struct {
	InputType          *string
	ExpectedOutputType *string
	Input              *string
	ExpectedOutput     *string

	ShowInput          *bool
	ShowExpectedOutput *bool
	ShowUserOutput     *bool

	CPUTimeLimitSeconds *int
	MemUsageLimitBytes  *int64
	StackLimitBytes     *int64
	FileSizeLimitBytes  *int64

	InputFile          *UploadedFile
	ExpectedOutputFile *UploadedFile

	Files         []UploadedFile
	FilesProvided bool
}

type resolvedFile struct {
	Name        string
	Data        []byte
	ContentType string
}

type resolvedTestCase struct {
	Input              string
	InputType          string
	ExpectedOutput     string
	ExpectedOutputType string

	ShowInput          bool
	ShowExpectedOutput bool
	ShowUserOutput     bool

	CPUTimeLimitSeconds int
	MemUsageLimitBytes  int64
	StackLimitBytes     int64
	FileSizeLimitBytes  int64

	// InputObject/OutputObject are non-nil when the object must be (re)uploaded
	// to <case_id>/in or <case_id>/out.
	InputObject  *UploadedFile
	OutputObject *UploadedFile

	Files        []resolvedFile
	FilesChanged bool
}

func textObject(text string) *UploadedFile {
	return &UploadedFile{
		ContentType: "text/plain; charset=utf-8",
		Data:        []byte(text),
	}
}

func validateIOType(value string) error {
	if value != IOTypeText && value != IOTypeFile {
		return fmt.Errorf("%w: type must be %q or %q", ErrInvalidTestCase, IOTypeText, IOTypeFile)
	}
	return nil
}

/*
ResolveTestCaseInput merges a partial request over an existing test case (nil
for create) and validates the result. It is a pure function so it can be tested
without a database.
*/
func ResolveTestCaseInput(
	in *TestCaseInput, existing *models.TestCase,
) (*resolvedTestCase, error) {
	res := &resolvedTestCase{}

	if existing != nil {
		res.Input = existing.Input
		res.InputType = existing.InputType
		res.ExpectedOutput = existing.ExpectedOutput
		res.ExpectedOutputType = existing.ExpectedOutputType
		res.ShowInput = existing.ShowInput
		res.ShowExpectedOutput = existing.ShowExpectedOutput
		res.ShowUserOutput = existing.ShowUserOutput
		res.CPUTimeLimitSeconds = existing.CPUTimeLimitSeconds
		res.MemUsageLimitBytes = existing.MemUsageLimitBytes
		res.StackLimitBytes = existing.StackLimitBytes
		res.FileSizeLimitBytes = existing.FileSizeLimitBytes
	} else {
		// Matches the schema defaults.
		res.ShowInput = true
		res.ShowExpectedOutput = true
		res.ShowUserOutput = true
	}

	if in.InputType != nil {
		res.InputType = *in.InputType
	}
	if res.InputType == "" {
		return nil, fmt.Errorf("%w: input_type is required", ErrInvalidTestCase)
	}
	if err := validateIOType(res.InputType); err != nil {
		return nil, err
	}

	if in.ExpectedOutputType != nil {
		res.ExpectedOutputType = *in.ExpectedOutputType
	}
	if res.ExpectedOutputType == "" {
		return nil, fmt.Errorf("%w: expected_output_type is required", ErrInvalidTestCase)
	}
	if err := validateIOType(res.ExpectedOutputType); err != nil {
		return nil, err
	}

	// Input object.
	switch res.InputType {
	case IOTypeText:
		if in.Input != nil {
			res.Input = *in.Input
			res.InputObject = textObject(*in.Input)
		} else if existing == nil || existing.InputType != IOTypeText {
			return nil, fmt.Errorf("%w: input is required for text input", ErrInvalidTestCase)
		}
	case IOTypeFile:
		if in.InputFile != nil {
			res.Input = storage.FileBasename(in.InputFile.Filename)
			res.InputObject = in.InputFile
		} else if existing == nil || existing.InputType != IOTypeFile {
			return nil, fmt.Errorf("%w: input_file is required for file input", ErrInvalidTestCase)
		}
	}

	// Expected output object.
	switch res.ExpectedOutputType {
	case IOTypeText:
		if in.ExpectedOutput != nil {
			res.ExpectedOutput = *in.ExpectedOutput
			res.OutputObject = textObject(*in.ExpectedOutput)
		} else if existing == nil || existing.ExpectedOutputType != IOTypeText {
			return nil, fmt.Errorf("%w: expected_output is required for text output", ErrInvalidTestCase)
		}
	case IOTypeFile:
		if in.ExpectedOutputFile != nil {
			res.ExpectedOutput = storage.FileBasename(in.ExpectedOutputFile.Filename)
			res.OutputObject = in.ExpectedOutputFile
		} else if existing == nil || existing.ExpectedOutputType != IOTypeFile {
			return nil, fmt.Errorf("%w: expected_output_file is required for file output", ErrInvalidTestCase)
		}
	}

	if in.ShowInput != nil {
		res.ShowInput = *in.ShowInput
	}
	if in.ShowExpectedOutput != nil {
		res.ShowExpectedOutput = *in.ShowExpectedOutput
	}
	if in.ShowUserOutput != nil {
		res.ShowUserOutput = *in.ShowUserOutput
	}

	var err error
	if res.CPUTimeLimitSeconds, err = resolveInt(
		in.CPUTimeLimitSeconds, existingCPUTime(existing),
	); err != nil {
		return nil, err
	}

	if res.MemUsageLimitBytes, err = resolveInt64(
		in.MemUsageLimitBytes, existingMem(existing),
	); err != nil {
		return nil, err
	}
	if res.StackLimitBytes, err = resolveInt64(
		in.StackLimitBytes, existingStack(existing),
	); err != nil {
		return nil, err
	}
	if res.FileSizeLimitBytes, err = resolveInt64(
		in.FileSizeLimitBytes, existingFileSize(existing),
	); err != nil {
		return nil, err
	}

	if in.FilesProvided {
		res.FilesChanged = true
		res.Files = make([]resolvedFile, 0, len(in.Files))
		seen := make(map[string]struct{}, len(in.Files))
		for _, f := range in.Files {
			name := storage.FileBasename(f.Filename)
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			res.Files = append(res.Files, resolvedFile{
				Name:        name,
				Data:        f.Data,
				ContentType: f.ContentType,
			})
		}
	}

	return res, nil
}

func existingCPUTime(existing *models.TestCase) int {
	if existing == nil {
		return 0
	}
	return existing.CPUTimeLimitSeconds
}

func existingMem(existing *models.TestCase) int64 {
	if existing == nil {
		return 0
	}
	return existing.MemUsageLimitBytes
}

func existingStack(existing *models.TestCase) int64 {
	if existing == nil {
		return 0
	}
	return existing.StackLimitBytes
}

func existingFileSize(existing *models.TestCase) int64 {
	if existing == nil {
		return 0
	}
	return existing.FileSizeLimitBytes
}

func resolveInt(provided *int, fallback int) (int, error) {
	if provided == nil {
		return fallback, nil
	}
	if *provided < 0 {
		return 0, fmt.Errorf("%w: value cannot be negative", ErrInvalidTestCase)
	}
	return *provided, nil
}

func resolveInt64(provided *int64, fallback int64) (int64, error) {
	if provided == nil {
		return fallback, nil
	}
	if *provided < 0 {
		return 0, fmt.Errorf("%w: value cannot be negative", ErrInvalidTestCase)
	}
	return *provided, nil
}

/*
loadTestCase fetches a single test case scoped to its exercise.
*/
func loadTestCase(
	ctx context.Context, caseID, exerciseID int64,
) (*models.TestCase, error) {
	tc := models.TestCase{}
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, exercise_id, input, input_type::text, show_input,
		       expected_output, expected_output_type::text,
		       show_expected_output, show_user_output, cpu_time_limit_seconds,
		       mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes
		FROM exercises_test_cases
		WHERE id = $1 AND exercise_id = $2`, caseID, exerciseID,
	).Scan(
		&tc.ID, &tc.ExerciseID, &tc.Input, &tc.InputType, &tc.ShowInput,
		&tc.ExpectedOutput, &tc.ExpectedOutputType, &tc.ShowExpectedOutput,
		&tc.ShowUserOutput, &tc.CPUTimeLimitSeconds, &tc.MemUsageLimitBytes,
		&tc.StackLimitBytes, &tc.FileSizeLimitBytes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTestCaseNotFound
		}
		slog.ErrorContext(ctx, "error fetching test case",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	return &tc, nil
}

/*
loadTestCaseFiles returns the extra files of the given test cases, keyed by
test-case id.
*/
func loadTestCaseFiles(
	ctx context.Context, caseIDs []int64,
) (map[int64][]models.TestCaseFile, error) {
	files := make(map[int64][]models.TestCaseFile)
	if len(caseIDs) == 0 {
		return files, nil
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, exercise_test_case_id, path
		FROM exercises_test_cases_files
		WHERE exercise_test_case_id = ANY($1)
		ORDER BY exercise_test_case_id, id`, pq.Array(caseIDs))
	if err != nil {
		slog.ErrorContext(ctx, "error fetching test case files",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			file   models.TestCaseFile
			caseID int64
		)
		if err := rows.Scan(&file.ID, &caseID, &file.Path); err != nil {
			slog.ErrorContext(ctx, "error scanning test case file",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		files[caseID] = append(files[caseID], file)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return files, nil
}
