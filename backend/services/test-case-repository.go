package services

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
)

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
	if !acc.canAuthor {
		if !acc.isEnrolled {
			return nil, ErrNotEnrolled
		}
		if !exerciseVisible(acc.exercise, nowFunc()) {
			return nil, ErrExerciseNotFound
		}
	}

	authoringID := acc.authoringExerciseID()

	rows, err := database.DB.QueryContext(ctx, `
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

	if !acc.canAuthor {
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
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	res, err := ResolveTestCaseInput(in, nil)
	if err != nil {
		return nil, err
	}

	tx, err := database.DB.BeginTx(ctx, nil)
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
		if err := storage.PutCaseObject(
			ctx, key, bytes.NewReader(obj.Data), int64(len(obj.Data)), obj.ContentType,
		); err != nil {
			return err
		}
		uploaded = append(uploaded, key)
		return nil
	}

	if err := upload(storage.CaseInputKey(caseID), res.InputObject); err != nil {
		slog.ErrorContext(ctx, "failed to upload test case input",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}
	if err := upload(storage.CaseOutputKey(caseID), res.OutputObject); err != nil {
		slog.ErrorContext(ctx, "failed to upload test case output",
			slog.Int64("test_case_id", caseID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	files := make([]models.TestCaseFile, 0, len(res.Files))
	for _, f := range res.Files {
		key := storage.CaseFileKey(caseID, f.Name)
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
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
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

	tx, err := database.DB.BeginTx(ctx, nil)
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

	// Objects are replaced in place, but the replacer backs up their previous
	// content first so a failure before the transaction commits can restore it.
	replacer := storage.NewObjectReplacer(ctx, storage.CaseOps)

	if res.InputObject != nil {
		if err := replacer.Replace(storage.CaseInputKey(caseID), res.InputObject.Data, res.InputObject.ContentType); err != nil {
			slog.ErrorContext(ctx, "failed to upload test case input",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			replacer.Restore()
			return nil, ErrServer
		}
	}
	if res.OutputObject != nil {
		if err := replacer.Replace(storage.CaseOutputKey(caseID), res.OutputObject.Data, res.OutputObject.ContentType); err != nil {
			slog.ErrorContext(ctx, "failed to upload test case output",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			replacer.Restore()
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
			replacer.Restore()
			return nil, ErrServer
		}

		newFiles = make([]models.TestCaseFile, 0, len(res.Files))
		for _, f := range res.Files {
			key := storage.CaseFileKey(caseID, f.Name)
			if err := replacer.Replace(key, f.Data, f.ContentType); err != nil {
				slog.ErrorContext(ctx, "failed to upload test case file",
					slog.Int64("test_case_id", caseID),
					slog.String("error", err.Error()),
				)
				replacer.Restore()
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
				replacer.Restore()
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
		replacer.Restore()
		return nil, ErrServer
	}

	// The transaction is durable, so the backups are no longer needed.
	replacer.Discard()

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
			deleteCaseObjectsBestEffort(ctx, []string{storage.CaseFileKey(caseID, f.Path)})
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
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return err
	}
	authoringID := acc.authoringExerciseID()

	files, err := loadTestCaseFiles(ctx, []int64{caseID})
	if err != nil {
		return err
	}

	result, err := database.DB.ExecContext(ctx, `
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

	keys := []string{storage.CaseInputKey(caseID), storage.CaseOutputKey(caseID)}
	for _, f := range files[caseID] {
		keys = append(keys, storage.CaseFileKey(caseID, f.Path))
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
		if err := storage.DeleteCaseObject(ctx, key); err != nil {
			slog.ErrorContext(ctx, "failed to clean up case object",
				slog.String("s3_key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}
