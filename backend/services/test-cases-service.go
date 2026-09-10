package services

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/models"

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
			res.Input = fileBasename(in.InputFile.Filename)
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
			res.ExpectedOutput = fileBasename(in.ExpectedOutputFile.Filename)
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
			name := fileBasename(f.Filename)
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
	err := DB.QueryRowContext(ctx, `
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

	rows, err := DB.QueryContext(ctx, `
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

/*
ListTestCases returns an exercise's test cases. Owners get the full rows;
enrolled students get the same rows with hidden fields blanked out.
*/
func ListTestCases(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.TestCase, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if !acc.isOwner {
		if !acc.isEnrolled {
			return nil, ErrNotEnrolled
		}
		if !exerciseVisible(acc.exercise, nowFunc()) {
			return nil, ErrExerciseNotFound
		}
	}

	authoringID := acc.authoringExerciseID()

	rows, err := DB.QueryContext(ctx, `
		SELECT id, exercise_id, input, input_type::text, show_input,
		       expected_output, expected_output_type::text,
		       show_expected_output, show_user_output, cpu_time_limit_seconds,
		       mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes
		FROM exercises_test_cases
		WHERE exercise_id = $1
		ORDER BY id`, authoringID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching test cases",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	cases := make([]models.TestCase, 0, 8)
	ids := make([]int64, 0, 8)
	for rows.Next() {
		var tc models.TestCase
		if err := rows.Scan(
			&tc.ID, &tc.ExerciseID, &tc.Input, &tc.InputType, &tc.ShowInput,
			&tc.ExpectedOutput, &tc.ExpectedOutputType, &tc.ShowExpectedOutput,
			&tc.ShowUserOutput, &tc.CPUTimeLimitSeconds, &tc.MemUsageLimitBytes,
			&tc.StackLimitBytes, &tc.FileSizeLimitBytes,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning test case",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		tc.Files = []models.TestCaseFile{}
		cases = append(cases, tc)
		ids = append(ids, tc.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	files, err := loadTestCaseFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range cases {
		if f, ok := files[cases[i].ID]; ok {
			cases[i].Files = f
		}
	}

	if !acc.isOwner {
		for i := range cases {
			if !cases[i].ShowInput {
				cases[i].Input = ""
			}
			if !cases[i].ShowExpectedOutput {
				cases[i].ExpectedOutput = ""
			}
		}
	}

	return cases, nil
}

/*
CreateTestCase creates a test case for an exercise owned by the caller. The row
is inserted first (to obtain the id used in the S3 keys), the objects are
uploaded, and only then is the transaction committed.
*/
func CreateTestCase(
	ctx context.Context, exerciseID int64, in *TestCaseInput, claims map[string]any,
) (*models.TestCase, error) {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	res, err := ResolveTestCaseInput(in, nil)
	if err != nil {
		return nil, err
	}

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing test case transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	var caseID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO exercises_test_cases
		    (exercise_id, input, input_type, show_input, expected_output,
		     expected_output_type, show_expected_output, show_user_output,
		     cpu_time_limit_seconds, mem_usage_limit_bytes, stack_limit_bytes,
		     file_size_limit_bytes)
		VALUES ($1, $2, $3::input_output_type_t, $4, $5,
		        $6::input_output_type_t, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		authoringID, res.Input, res.InputType, res.ShowInput, res.ExpectedOutput,
		res.ExpectedOutputType, res.ShowExpectedOutput, res.ShowUserOutput,
		res.CPUTimeLimitSeconds, res.MemUsageLimitBytes, res.StackLimitBytes,
		res.FileSizeLimitBytes,
	).Scan(&caseID)
	if err != nil {
		slog.ErrorContext(ctx, "error inserting test case",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	uploaded := make([]string, 0, 4)

	upload := func(key string, obj *UploadedFile) error {
		if err := PutCaseObject(
			ctx, key, bytes.NewReader(obj.Data), int64(len(obj.Data)), obj.ContentType,
		); err != nil {
			return err
		}
		uploaded = append(uploaded, key)
		return nil
	}

	if err := upload(CaseInputKey(caseID), res.InputObject); err != nil {
		slog.ErrorContext(ctx, "failed to upload test case input",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}
	if err := upload(CaseOutputKey(caseID), res.OutputObject); err != nil {
		slog.ErrorContext(ctx, "failed to upload test case output",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	files := make([]models.TestCaseFile, 0, len(res.Files))
	for _, f := range res.Files {
		key := CaseFileKey(caseID, f.Name)
		if err := upload(key, &UploadedFile{
			Filename: f.Name, ContentType: f.ContentType, Data: f.Data,
		}); err != nil {
			slog.ErrorContext(ctx, "failed to upload test case file",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}

		var fileID int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO exercises_test_cases_files (exercise_test_case_id, path)
			VALUES ($1, $2)
			RETURNING id`, caseID, f.Name,
		).Scan(&fileID); err != nil {
			slog.ErrorContext(ctx, "error inserting test case file",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}
		files = append(files, models.TestCaseFile{ID: fileID, Path: f.Name})
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing test case transaction",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	return testCaseFromResolved(caseID, authoringID, res, files), nil
}

/*
UpdateTestCase applies a partial update to a test case. Only the provided
fields (and files) change.
*/
func UpdateTestCase(
	ctx context.Context, exerciseID, caseID int64, in *TestCaseInput,
	claims map[string]any,
) (*models.TestCase, error) {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	existing, err := loadTestCase(ctx, caseID, authoringID)
	if err != nil {
		return nil, err
	}

	res, err := ResolveTestCaseInput(in, existing)
	if err != nil {
		return nil, err
	}

	existingFiles, err := loadTestCaseFiles(ctx, []int64{caseID})
	if err != nil {
		return nil, err
	}
	oldFiles := existingFiles[caseID]
	if oldFiles == nil {
		oldFiles = []models.TestCaseFile{}
	}

	var newFiles []models.TestCaseFile
	if !res.FilesChanged {
		newFiles = oldFiles
	}

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing test case update transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE exercises_test_cases
		SET input = $1, input_type = $2::input_output_type_t, show_input = $3,
		    expected_output = $4, expected_output_type = $5::input_output_type_t,
		    show_expected_output = $6, show_user_output = $7,
		    cpu_time_limit_seconds = $8, mem_usage_limit_bytes = $9,
		    stack_limit_bytes = $10, file_size_limit_bytes = $11,
		    updated_at = now()
		WHERE id = $12 AND exercise_id = $13`,
		res.Input, res.InputType, res.ShowInput, res.ExpectedOutput,
		res.ExpectedOutputType, res.ShowExpectedOutput, res.ShowUserOutput,
		res.CPUTimeLimitSeconds, res.MemUsageLimitBytes, res.StackLimitBytes,
		res.FileSizeLimitBytes, caseID, authoringID,
	); err != nil {
		slog.ErrorContext(ctx, "error updating test case",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	uploaded := make([]string, 0, 4)

	upload := func(key string, obj *UploadedFile) error {
		if err := PutCaseObject(
			ctx, key, bytes.NewReader(obj.Data), int64(len(obj.Data)), obj.ContentType,
		); err != nil {
			return err
		}
		uploaded = append(uploaded, key)
		return nil
	}

	if res.InputObject != nil {
		if err := upload(CaseInputKey(caseID), res.InputObject); err != nil {
			slog.ErrorContext(ctx, "failed to upload test case input",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}
	}
	if res.OutputObject != nil {
		if err := upload(CaseOutputKey(caseID), res.OutputObject); err != nil {
			slog.ErrorContext(ctx, "failed to upload test case output",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}
	}

	if res.FilesChanged {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM exercises_test_cases_files WHERE exercise_test_case_id = $1`,
			caseID,
		); err != nil {
			slog.ErrorContext(ctx, "error clearing test case files",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}

		newFiles = make([]models.TestCaseFile, 0, len(res.Files))
		for _, f := range res.Files {
			key := CaseFileKey(caseID, f.Name)
			if err := upload(key, &UploadedFile{
				Filename: f.Name, ContentType: f.ContentType, Data: f.Data,
			}); err != nil {
				slog.ErrorContext(ctx, "failed to upload test case file",
					slog.Int64("test_case_id", caseID),
					slog.String("error", err.Error()),
				)
				deleteCaseObjectsBestEffort(ctx, uploaded)
				return nil, ErrServer
			}

			var fileID int64
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO exercises_test_cases_files (exercise_test_case_id, path)
				VALUES ($1, $2)
				RETURNING id`, caseID, f.Name,
			).Scan(&fileID); err != nil {
				slog.ErrorContext(ctx, "error inserting test case file",
					slog.Int64("test_case_id", caseID),
					slog.String("error", err.Error()),
				)
				deleteCaseObjectsBestEffort(ctx, uploaded)
				return nil, ErrServer
			}
			newFiles = append(newFiles, models.TestCaseFile{ID: fileID, Path: f.Name})
		}
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing test case update",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	if res.FilesChanged {
		// The old files are no longer referenced; drop their objects, but keep
		// the ones that were replaced with the same name.
		keep := make(map[string]struct{}, len(newFiles))
		for _, f := range newFiles {
			keep[f.Path] = struct{}{}
		}
		for _, f := range oldFiles {
			if _, ok := keep[f.Path]; ok {
				continue
			}
			deleteCaseObjectsBestEffort(ctx, []string{CaseFileKey(caseID, f.Path)})
		}
	}

	return testCaseFromResolved(caseID, authoringID, res, newFiles), nil
}

/*
DeleteTestCase removes a test case and best-effort deletes its S3 objects.
*/
func DeleteTestCase(
	ctx context.Context, exerciseID, caseID int64, claims map[string]any,
) error {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return err
	}
	authoringID := acc.authoringExerciseID()

	files, err := loadTestCaseFiles(ctx, []int64{caseID})
	if err != nil {
		return err
	}

	result, err := DB.ExecContext(ctx, `
		DELETE FROM exercises_test_cases
		WHERE id = $1 AND exercise_id = $2`, caseID, authoringID)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting test case",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return ErrServer
	}
	if affected == 0 {
		return ErrTestCaseNotFound
	}

	keys := []string{CaseInputKey(caseID), CaseOutputKey(caseID)}
	for _, f := range files[caseID] {
		keys = append(keys, CaseFileKey(caseID, f.Path))
	}
	deleteCaseObjectsBestEffort(ctx, keys)

	return nil
}

func testCaseFromResolved(
	caseID, exerciseID int64, res *resolvedTestCase, files []models.TestCaseFile,
) *models.TestCase {
	if files == nil {
		files = []models.TestCaseFile{}
	}
	return &models.TestCase{
		ID:                  caseID,
		ExerciseID:          exerciseID,
		Input:               res.Input,
		InputType:           res.InputType,
		ShowInput:           res.ShowInput,
		ExpectedOutput:      res.ExpectedOutput,
		ExpectedOutputType:  res.ExpectedOutputType,
		ShowExpectedOutput:  res.ShowExpectedOutput,
		ShowUserOutput:      res.ShowUserOutput,
		CPUTimeLimitSeconds: res.CPUTimeLimitSeconds,
		MemUsageLimitBytes:  res.MemUsageLimitBytes,
		StackLimitBytes:     res.StackLimitBytes,
		FileSizeLimitBytes:  res.FileSizeLimitBytes,
		Files:               files,
	}
}

/*
deleteCaseObjectsBestEffort removes uploaded case objects, logging (but not
returning) failures.
*/
func deleteCaseObjectsBestEffort(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := DeleteCaseObject(ctx, key); err != nil {
			slog.ErrorContext(ctx, "failed to clean up case object",
				slog.String("s3_key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}

/*
ListCompilationFiles returns the compilation files of an exercise (owner only).
*/
func ListCompilationFiles(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.CompilationFile, error) {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	return queryCompilationFiles(ctx, acc.authoringExerciseID())
}

func queryCompilationFiles(
	ctx context.Context, exerciseID int64,
) ([]models.CompilationFile, error) {
	rows, err := DB.QueryContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_compilation_files
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching compilation files",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	files := make([]models.CompilationFile, 0, 4)
	for rows.Next() {
		var f models.CompilationFile
		if err := rows.Scan(&f.ID, &f.ExerciseID, &f.Path); err != nil {
			return nil, ErrServer
		}
		f.Filename = f.Path
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return files, nil
}

/*
GetCompilationFile returns a single compilation file (owner only).
*/
func GetCompilationFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) (*models.CompilationFile, error) {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}

	var f models.CompilationFile
	err = DB.QueryRowContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_compilation_files
		WHERE id = $1 AND exercise_id = $2`,
		fileID, acc.authoringExerciseID(),
	).Scan(&f.ID, &f.ExerciseID, &f.Path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCompilationFileNotFound
		}
		slog.ErrorContext(ctx, "error fetching compilation file",
			slog.Int64("compilation_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	f.Filename = f.Path
	return &f, nil
}

/*
CreateCompilationFile stores a compilation file for an exercise owned by the
caller. A file with the same path is replaced.
*/
func CreateCompilationFile(
	ctx context.Context, exerciseID int64, file UploadedFile, claims map[string]any,
) (*models.CompilationFile, error) {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	name := fileBasename(file.Filename)
	key := CompilationFileKey(authoringID, name)

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing compilation file transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM exercises_compilation_files
		WHERE exercise_id = $1 AND path = $2`, authoringID, name,
	); err != nil {
		slog.ErrorContext(ctx, "error replacing compilation file row",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	var fileID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exercises_compilation_files (exercise_id, path)
		VALUES ($1, $2)
		RETURNING id`, authoringID, name,
	).Scan(&fileID); err != nil {
		slog.ErrorContext(ctx, "error inserting compilation file",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if err := PutFileObject(
		ctx, key, bytes.NewReader(file.Data), int64(len(file.Data)), file.ContentType,
	); err != nil {
		slog.ErrorContext(ctx, "failed to upload compilation file",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		deleteFileObjectBestEffort(ctx, key)
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing compilation file",
			slog.String("error", err.Error()),
		)
		deleteFileObjectBestEffort(ctx, key)
		return nil, ErrServer
	}

	return &models.CompilationFile{ID: fileID, ExerciseID: authoringID, Path: name, Filename: name}, nil
}

/*
DeleteCompilationFile removes a compilation file (row and S3 object).
*/
func DeleteCompilationFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) error {
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return err
	}
	authoringID := acc.authoringExerciseID()

	var path string
	err = DB.QueryRowContext(ctx, `
		DELETE FROM exercises_compilation_files
		WHERE id = $1 AND exercise_id = $2
		RETURNING path`, fileID, authoringID,
	).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCompilationFileNotFound
		}
		slog.ErrorContext(ctx, "error deleting compilation file",
			slog.Int64("compilation_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	deleteFileObjectBestEffort(ctx, CompilationFileKey(authoringID, path))
	return nil
}

func deleteFileObjectBestEffort(ctx context.Context, key string) {
	if err := DeleteFileObject(ctx, key); err != nil {
		slog.ErrorContext(ctx, "failed to clean up file object",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
	}
}

/*
ListAttachedFiles returns an exercise's attachments. Owners and enrolled
students may read them.
*/
func ListAttachedFiles(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.AttachedFile, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}
	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if !acc.isOwner && !acc.isEnrolled {
		return nil, ErrNotEnrolled
	}
	return queryAttachedFiles(ctx, exerciseID)
}

func queryAttachedFiles(
	ctx context.Context, exerciseID int64,
) ([]models.AttachedFile, error) {
	rows, err := DB.QueryContext(ctx, `
		SELECT id, exercise_id, path
		FROM exercises_attached_files
		WHERE exercise_id = $1
		ORDER BY id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching attached files",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	files := make([]models.AttachedFile, 0, 4)
	for rows.Next() {
		var f models.AttachedFile
		if err := rows.Scan(&f.ID, &f.ExerciseID, &f.Path); err != nil {
			return nil, ErrServer
		}
		f.Filename = f.Path
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return files, nil
}

/*
CreateAttachedFile stores an attachment for an exercise owned by the caller
under the files bucket (`attachments/<exercise_id>/<basename>`).
*/
func CreateAttachedFile(
	ctx context.Context, exerciseID int64, file UploadedFile, claims map[string]any,
) (*models.AttachedFile, error) {
	if _, err := requireExerciseOwner(ctx, exerciseID, claims); err != nil {
		return nil, err
	}

	name := fileBasename(file.Filename)
	key := AttachmentKey(exerciseID, name)

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing attached file transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM exercises_attached_files
		WHERE exercise_id = $1 AND path = $2`, exerciseID, name,
	); err != nil {
		slog.ErrorContext(ctx, "error replacing attached file row",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	var fileID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exercises_attached_files (exercise_id, path)
		VALUES ($1, $2)
		RETURNING id`, exerciseID, name,
	).Scan(&fileID); err != nil {
		slog.ErrorContext(ctx, "error inserting attached file",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if err := PutFileObject(
		ctx, key, bytes.NewReader(file.Data), int64(len(file.Data)), file.ContentType,
	); err != nil {
		slog.ErrorContext(ctx, "failed to upload attached file",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		deleteFileObjectBestEffort(ctx, key)
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing attached file",
			slog.String("error", err.Error()),
		)
		deleteFileObjectBestEffort(ctx, key)
		return nil, ErrServer
	}

	return &models.AttachedFile{ID: fileID, ExerciseID: exerciseID, Path: name, Filename: name}, nil
}

/*
DeleteAttachedFile removes an attachment (row and S3 object).
*/
func DeleteAttachedFile(
	ctx context.Context, exerciseID, fileID int64, claims map[string]any,
) error {
	if _, err := requireExerciseOwner(ctx, exerciseID, claims); err != nil {
		return err
	}

	var path string
	err := DB.QueryRowContext(ctx, `
		DELETE FROM exercises_attached_files
		WHERE id = $1 AND exercise_id = $2
		RETURNING path`, fileID, exerciseID,
	).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAttachedFileNotFound
		}
		slog.ErrorContext(ctx, "error deleting attached file",
			slog.Int64("attached_file_id", fileID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	deleteFileObjectBestEffort(ctx, AttachmentKey(exerciseID, path))
	return nil
}
