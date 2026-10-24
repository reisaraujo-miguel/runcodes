package services

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
	"github.com/runcodes-icmc/runcodes/validation"
)

const (
	// offeringArchiveFormat and offeringArchiveVersion identify the manifest.
	// An importer refuses a format it does not know rather than guessing.
	offeringArchiveFormat  = "runcodes-offering"
	offeringArchiveVersion = 1

	// MaxOfferingArchiveBytes bounds the compressed upload.
	MaxOfferingArchiveBytes = 256 << 20
	// MaxOfferingExpandedBytes bounds the sum of every file in the archive.
	MaxOfferingExpandedBytes = 512 << 20
	// MaxOfferingArchiveEntries bounds the number of zip entries.
	MaxOfferingArchiveEntries = 20000
	// MaxImportedExercises and MaxImportedTestCases bound one import, so a
	// hand-crafted manifest cannot create an unbounded number of rows.
	MaxImportedExercises = 500
	MaxImportedTestCases = 5000
	// maxImportFileBytes bounds one exercise file (material, compilation file or
	// test-case file).
	maxImportFileBytes = MaxBatchCaseFileBytes
)

// offeringArchiveManifest is the JSON index of an exported offering. The files
// themselves live next to it in the zip; the manifest only names them.
type offeringArchiveManifest struct {
	Format     string                    `json:"format"`
	Version    int                       `json:"version"`
	ExportedAt time.Time                 `json:"exported_at"`
	Offering   offeringArchiveOffering   `json:"offering"`
	Exercises  []offeringArchiveExercise `json:"exercises"`
}

type offeringArchiveOffering struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	EndDate         string `json:"end_date"`
	VisibleToEnroll bool   `json:"visible_to_enroll"`
}

type offeringArchiveExercise struct {
	Title                     string                    `json:"title"`
	Description               string                    `json:"description"`
	Deadline                  time.Time                 `json:"deadline"`
	OpenDate                  time.Time                 `json:"open_date"`
	ShowBeforeOpenDate        bool                      `json:"show_before_open_date"`
	Removed                   bool                      `json:"removed"`
	AllowedFileTypeExtensions []string                  `json:"allowed_file_type_extensions"`
	TestCases                 []offeringArchiveTestCase `json:"test_cases"`
	CompilationFiles          []string                  `json:"compilation_files"`
	AttachedFiles             []string                  `json:"attached_files"`
}

type offeringArchiveTestCase struct {
	Input               string   `json:"input"`
	InputType           string   `json:"input_type"`
	ShowInput           bool     `json:"show_input"`
	ExpectedOutput      string   `json:"expected_output"`
	ExpectedOutputType  string   `json:"expected_output_type"`
	ShowExpectedOutput  bool     `json:"show_expected_output"`
	ShowUserOutput      bool     `json:"show_user_output"`
	CPUTimeLimitSeconds int      `json:"cpu_time_limit_seconds"`
	MemUsageLimitBytes  int64    `json:"mem_usage_limit_bytes"`
	StackLimitBytes     int64    `json:"stack_limit_bytes"`
	FileSizeLimitBytes  int64    `json:"file_size_limit_bytes"`
	Files               []string `json:"files"`
}

// OfferingImportOptions are the fields the professor may override when turning
// an archive into a new class. Both are optional: absent values come from the
// manifest.
type OfferingImportOptions struct {
	Name        string
	Description *string
	EndDate     *time.Time
}

// offeringArchive is a parsed archive: the manifest plus a lookup from entry
// name to its compressed content.
type offeringArchive struct {
	manifest *offeringArchiveManifest
	entries  map[string]*zip.File
}

/*
ExportOfferingArchive packages a whole offering — metadata, exercises, test
cases, compilation files and materials — into a zip that ImportOfferingArchive
can turn back into a new class. Submissions are deliberately left out: the point
is to reuse the course material, not the previous class's answers.
*/
func ExportOfferingArchive(
	ctx context.Context, offeringID int64, claims map[string]any,
) (filename string, data []byte, err error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return "", nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return "", nil, err
	}
	if !rel.CanAuthor() {
		return "", nil, ErrOfferingNotFound
	}

	manifest := &offeringArchiveManifest{
		Format:     offeringArchiveFormat,
		Version:    offeringArchiveVersion,
		ExportedAt: time.Now().UTC(),
		Offering: offeringArchiveOffering{
			Name:            rel.Offering.Name,
			Description:     rel.Offering.Description,
			EndDate:         rel.Offering.EndDate,
			VisibleToEnroll: rel.Offering.VisibleToEnroll,
		},
		Exercises: []offeringArchiveExercise{},
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, title, COALESCE(description, ''), deadline, open_date,
		       show_before_open_date, removed, ghost, real_id
		FROM exercises
		WHERE offering_id = $1
		ORDER BY open_date, id`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error listing exercises for export",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return "", nil, ErrServer
	}

	type exportExercise struct {
		manifest    offeringArchiveExercise
		id          int64
		authoringID int64
	}
	exercises := make([]exportExercise, 0, 8)
	for rows.Next() {
		var (
			exercise    exportExercise
			ghost       bool
			realID      sql.NullInt64
			description string
		)
		if err := rows.Scan(
			&exercise.id, &exercise.manifest.Title, &description,
			&exercise.manifest.Deadline, &exercise.manifest.OpenDate,
			&exercise.manifest.ShowBeforeOpenDate, &exercise.manifest.Removed,
			&ghost, &realID,
		); err != nil {
			rows.Close()
			slog.ErrorContext(ctx, "error scanning an exercise for export",
				slog.String("error", err.Error()),
			)
			return "", nil, ErrServer
		}
		exercise.manifest.Description = description
		exercise.authoringID = exercise.id
		if ghost && realID.Valid {
			exercise.authoringID = realID.Int64
		}
		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", nil, ErrServer
	}
	rows.Close()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for index := range exercises {
		exercise := &exercises[index]

		extensions, err := loadExerciseFileTypeExtensions(ctx, exercise.id)
		if err != nil {
			writer.Close()
			return "", nil, err
		}
		exercise.manifest.AllowedFileTypeExtensions = extensions

		cases, err := loadExportTestCases(ctx, exercise.authoringID)
		if err != nil {
			writer.Close()
			return "", nil, err
		}

		for caseIndex := range cases {
			testCase := &cases[caseIndex]
			base := fmt.Sprintf("exercises/%03d/testcases/%03d", index+1, caseIndex+1)

			exportCase := offeringArchiveTestCase{
				Input:               testCase.Input,
				InputType:           testCase.InputType,
				ShowInput:           testCase.ShowInput,
				ExpectedOutput:      testCase.ExpectedOutput,
				ExpectedOutputType:  testCase.ExpectedOutputType,
				ShowExpectedOutput:  testCase.ShowExpectedOutput,
				ShowUserOutput:      testCase.ShowUserOutput,
				CPUTimeLimitSeconds: testCase.CPUTimeLimitSeconds,
				MemUsageLimitBytes:  testCase.MemUsageLimitBytes,
				StackLimitBytes:     testCase.StackLimitBytes,
				FileSizeLimitBytes:  testCase.FileSizeLimitBytes,
				Files:               []string{},
			}

			input, err := loadCaseObject(
				ctx, testCase.ID, testCase.InputType, testCase.Input, false,
			)
			if err != nil {
				writer.Close()
				return "", nil, err
			}
			output, err := loadCaseObject(
				ctx, testCase.ID, testCase.ExpectedOutputType,
				testCase.ExpectedOutput, true,
			)
			if err != nil {
				writer.Close()
				return "", nil, err
			}
			if err := writeZipEntry(writer, base+"/in", input); err != nil {
				writer.Close()
				return "", nil, err
			}
			if err := writeZipEntry(writer, base+"/out", output); err != nil {
				writer.Close()
				return "", nil, err
			}

			for _, file := range testCase.Files {
				content, err := readCaseObject(
					ctx, storage.CaseFileKey(testCase.ID, file.Path),
				)
				if err != nil {
					writer.Close()
					return "", nil, err
				}
				if err := writeZipEntry(
					writer, base+"/files/"+storage.FileBasename(file.Path), content,
				); err != nil {
					writer.Close()
					return "", nil, err
				}
				exportCase.Files = append(exportCase.Files, storage.FileBasename(file.Path))
			}

			exercise.manifest.TestCases = append(exercise.manifest.TestCases, exportCase)
		}

		compilationFiles, err := loadExerciseFilePaths(
			ctx, "exercises_compilation_files", exercise.id,
		)
		if err != nil {
			writer.Close()
			return "", nil, err
		}
		for _, name := range compilationFiles {
			content, err := readFileObject(
				ctx, storage.CompilationFileKey(exercise.id, name),
			)
			if err != nil {
				writer.Close()
				return "", nil, err
			}
			if err := writeZipEntry(
				writer, fmt.Sprintf("exercises/%03d/compilation/%s", index+1, name),
				content,
			); err != nil {
				writer.Close()
				return "", nil, err
			}
			exercise.manifest.CompilationFiles = append(
				exercise.manifest.CompilationFiles, name,
			)
		}

		attachedFiles, err := loadExerciseFilePaths(
			ctx, "exercises_attached_files", exercise.id,
		)
		if err != nil {
			writer.Close()
			return "", nil, err
		}
		for _, name := range attachedFiles {
			content, err := readFileObject(
				ctx, storage.AttachmentKey(exercise.id, name),
			)
			if err != nil {
				writer.Close()
				return "", nil, err
			}
			if err := writeZipEntry(
				writer, fmt.Sprintf("exercises/%03d/attached/%s", index+1, name),
				content,
			); err != nil {
				writer.Close()
				return "", nil, err
			}
			exercise.manifest.AttachedFiles = append(
				exercise.manifest.AttachedFiles, name,
			)
		}

		manifest.Exercises = append(manifest.Exercises, exercise.manifest)
	}

	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		writer.Close()
		return "", nil, ErrServer
	}
	if err := writeZipEntry(writer, "manifest.json", encoded); err != nil {
		writer.Close()
		return "", nil, ErrServer
	}

	if err := writer.Close(); err != nil {
		slog.ErrorContext(ctx, "error finishing the offering archive",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return "", nil, ErrServer
	}

	return fmt.Sprintf("turma-%d.zip", offeringID), buffer.Bytes(), nil
}

func loadExerciseFileTypeExtensions(
	ctx context.Context, exerciseID int64,
) ([]string, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT aft.extension
		FROM exercises_allowed_file_types eaft
		JOIN allowed_file_types aft ON aft.id = eaft.allowed_file_type_id
		WHERE eaft.exercise_id = $1
		ORDER BY aft.extension`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading exercise file types for export",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	extensions := make([]string, 0, 4)
	for rows.Next() {
		var extension string
		if err := rows.Scan(&extension); err != nil {
			return nil, ErrServer
		}
		extensions = append(extensions, extension)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return extensions, nil
}

func loadExportTestCases(
	ctx context.Context, authoringID int64,
) ([]models.TestCase, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, exercise_id, input, input_type::text, show_input,
		       expected_output, expected_output_type::text,
		       show_expected_output, show_user_output, cpu_time_limit_seconds,
		       mem_usage_limit_bytes, stack_limit_bytes, file_size_limit_bytes
		FROM exercises_test_cases
		WHERE exercise_id = $1
		ORDER BY id`, authoringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading test cases for offering export",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	cases := make([]models.TestCase, 0, 8)
	ids := make([]int64, 0, 8)
	for rows.Next() {
		var testCase models.TestCase
		if err := rows.Scan(
			&testCase.ID, &testCase.ExerciseID, &testCase.Input,
			&testCase.InputType, &testCase.ShowInput,
			&testCase.ExpectedOutput, &testCase.ExpectedOutputType,
			&testCase.ShowExpectedOutput, &testCase.ShowUserOutput,
			&testCase.CPUTimeLimitSeconds, &testCase.MemUsageLimitBytes,
			&testCase.StackLimitBytes, &testCase.FileSizeLimitBytes,
		); err != nil {
			return nil, ErrServer
		}
		testCase.Files = []models.TestCaseFile{}
		cases = append(cases, testCase)
		ids = append(ids, testCase.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	files, err := loadTestCaseFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	for index := range cases {
		if list, ok := files[cases[index].ID]; ok {
			cases[index].Files = list
		}
	}
	return cases, nil
}

func loadExerciseFilePaths(
	ctx context.Context, table string, exerciseID int64,
) ([]string, error) {
	// table is a constant chosen by the caller below; it is never user input.
	query := "SELECT path FROM " + table + " WHERE exercise_id = $1 ORDER BY path"
	rows, err := database.DB.QueryContext(ctx, query, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading exercise files for export",
			slog.String("table", table),
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	names := make([]string, 0, 4)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, ErrServer
		}
		names = append(names, storage.FileBasename(path))
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return names, nil
}

func readFileObject(ctx context.Context, key string) ([]byte, error) {
	object, err := storage.GetFileObject(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			slog.WarnContext(ctx, "exercise file is missing for export",
				slog.String("key", key),
			)
			return nil, ErrImportFailed
		}
		slog.ErrorContext(ctx, "error reading an exercise file",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer object.Body.Close()

	data, err := io.ReadAll(io.LimitReader(object.Body, maxImportFileBytes+1))
	if err != nil {
		return nil, ErrServer
	}
	if len(data) > maxImportFileBytes {
		return nil, ErrArchiveTooLarge
	}
	return data, nil
}

/*
ParseOfferingArchive validates the manifest and every file it references. The
whole archive is checked before the first database write, so a missing file or an
unsafe name produces one clear error instead of a partially imported class.
*/
func ParseOfferingArchive(data []byte) (*offeringArchive, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: the archive is empty", ErrInvalidArchive)
	}
	if len(data) > MaxOfferingArchiveBytes {
		return nil, ErrArchiveTooLarge
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(reader.File) > MaxOfferingArchiveEntries {
		return nil, ErrArchiveTooLarge
	}

	archive := &offeringArchive{entries: make(map[string]*zip.File)}

	var expanded int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		segments, ok := normalizeArchivePath(file.Name)
		if !ok {
			return nil, fmt.Errorf("%w: invalid path %q", ErrInvalidTransfer, file.Name)
		}
		name := strings.Join(segments, "/")
		if name == "__MACOSX" || strings.HasPrefix(name, "__MACOSX/") ||
			name == ".DS_Store" || strings.HasSuffix(name, "/.DS_Store") {
			continue
		}
		if _, duplicate := archive.entries[name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate file %q", ErrInvalidTransfer, name)
		}
		expanded += int64(file.UncompressedSize64)
		if expanded > MaxOfferingExpandedBytes {
			return nil, ErrArchiveTooLarge
		}
		archive.entries[name] = file
	}

	manifestFile, ok := archive.entries["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("%w: manifest.json is missing", ErrInvalidTransfer)
	}
	manifestData, err := readZipEntry(manifestFile)
	if err != nil {
		return nil, err
	}
	var manifest offeringArchiveManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("%w: invalid manifest.json", ErrInvalidTransfer)
	}
	if manifest.Format != offeringArchiveFormat {
		return nil, fmt.Errorf("%w: unknown format %q", ErrInvalidTransfer, manifest.Format)
	}
	if manifest.Version != offeringArchiveVersion {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidTransfer, manifest.Version)
	}
	if err := validateOfferingManifest(&manifest); err != nil {
		return nil, err
	}
	archive.manifest = &manifest

	if err := archive.validateFiles(); err != nil {
		return nil, err
	}

	return archive, nil
}

func validateOfferingManifest(manifest *offeringArchiveManifest) error {
	name := strings.TrimSpace(manifest.Offering.Name)
	if err := validation.ValidateRequiredString(
		name, MaxOfferingNameLength,
	); err != nil {
		return fmt.Errorf("%w: invalid offering name", ErrInvalidTransfer)
	}
	manifest.Offering.Name = name
	if len(manifest.Exercises) > MaxImportedExercises {
		return fmt.Errorf("%w: too many exercises", ErrArchiveTooLarge)
	}

	totalCases := 0
	for index := range manifest.Exercises {
		exercise := &manifest.Exercises[index]
		exercise.Title = strings.TrimSpace(exercise.Title)
		if err := validation.ValidateRequiredString(
			exercise.Title, MaxExerciseTitleLength,
		); err != nil {
			return fmt.Errorf("%w: exercise %d has an invalid title",
				ErrInvalidTransfer, index+1)
		}
		if exercise.OpenDate.IsZero() || exercise.Deadline.IsZero() {
			return fmt.Errorf("%w: exercise %q has no dates",
				ErrInvalidTransfer, exercise.Title)
		}
		if exercise.Deadline.Before(exercise.OpenDate) {
			return fmt.Errorf("%w: exercise %q closes before it opens",
				ErrInvalidTransfer, exercise.Title)
		}
		if len(exercise.TestCases) > MaxImportedTestCases {
			return fmt.Errorf("%w: too many test cases in exercise %q",
				ErrArchiveTooLarge, exercise.Title)
		}
		totalCases += len(exercise.TestCases)
		if totalCases > MaxImportedTestCases {
			return fmt.Errorf("%w: too many test cases in the archive",
				ErrArchiveTooLarge)
		}

		for caseIndex := range exercise.TestCases {
			testCase := &exercise.TestCases[caseIndex]
			if err := validateIOType(testCase.InputType); err != nil {
				return fmt.Errorf("%w: test case %d of %q: %v",
					ErrInvalidTransfer, caseIndex+1, exercise.Title, err)
			}
			if err := validateIOType(testCase.ExpectedOutputType); err != nil {
				return fmt.Errorf("%w: test case %d of %q: %v",
					ErrInvalidTransfer, caseIndex+1, exercise.Title, err)
			}
			if testCase.CPUTimeLimitSeconds < 0 ||
				testCase.MemUsageLimitBytes < 0 ||
				testCase.StackLimitBytes < 0 ||
				testCase.FileSizeLimitBytes < 0 {
				return fmt.Errorf("%w: test case %d of %q has a negative limit",
					ErrInvalidTransfer, caseIndex+1, exercise.Title)
			}
			if err := validateArchiveFileNames(&testCase.Files); err != nil {
				return err
			}
		}

		if err := validateArchiveFileNames(&exercise.CompilationFiles); err != nil {
			return err
		}
		if err := validateArchiveFileNames(&exercise.AttachedFiles); err != nil {
			return err
		}
		seenExtensions := make(map[string]struct{}, len(exercise.AllowedFileTypeExtensions))
		for _, extension := range exercise.AllowedFileTypeExtensions {
			if _, duplicate := seenExtensions[extension]; duplicate {
				return fmt.Errorf("%w: duplicate file type %q in exercise %q",
					ErrInvalidTransfer, extension, exercise.Title)
			}
			seenExtensions[extension] = struct{}{}
		}
	}

	return nil
}

func validateArchiveFileNames(names *[]string) error {
	seen := make(map[string]struct{}, len(*names))
	cleaned := make([]string, 0, len(*names))
	for _, name := range *names {
		base := storage.FileBasename(name)
		if base == "" || base == "file" && strings.TrimSpace(name) == "" {
			return fmt.Errorf("%w: invalid file name %q", ErrInvalidTransfer, name)
		}
		if base != strings.TrimSpace(name) {
			return fmt.Errorf("%w: file name %q must be a plain file name",
				ErrInvalidTransfer, name)
		}
		if _, duplicate := seen[base]; duplicate {
			return fmt.Errorf("%w: duplicate file %q", ErrInvalidTransfer, base)
		}
		seen[base] = struct{}{}
		cleaned = append(cleaned, base)
	}
	*names = cleaned
	return nil
}

/*
validateFiles checks that every file the manifest names exists in the archive and
that its expanded size is bounded. It walks the same paths the import will use.
*/
func (a *offeringArchive) validateFiles() error {
	manifest := a.manifest
	for exerciseIndex, exercise := range manifest.Exercises {
		for caseIndex := range exercise.TestCases {
			testCase := &exercise.TestCases[caseIndex]
			base := fmt.Sprintf(
				"exercises/%03d/testcases/%03d", exerciseIndex+1, caseIndex+1,
			)
			for _, suffix := range []string{"/in", "/out"} {
				if err := a.requireEntry(base + suffix); err != nil {
					return err
				}
			}
			for _, name := range testCase.Files {
				if err := a.requireEntry(base + "/files/" + name); err != nil {
					return err
				}
			}
		}
		for _, name := range exercise.CompilationFiles {
			if err := a.requireEntry(
				fmt.Sprintf("exercises/%03d/compilation/%s", exerciseIndex+1, name),
			); err != nil {
				return err
			}
		}
		for _, name := range exercise.AttachedFiles {
			if err := a.requireEntry(
				fmt.Sprintf("exercises/%03d/attached/%s", exerciseIndex+1, name),
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *offeringArchive) requireEntry(name string) error {
	file, ok := a.entries[name]
	if !ok {
		return fmt.Errorf("%w: %s is missing from the archive",
			ErrInvalidTransfer, name)
	}
	if file.UncompressedSize64 > maxImportFileBytes {
		return ErrArchiveTooLarge
	}
	return nil
}

func (a *offeringArchive) read(name string) ([]byte, error) {
	file, ok := a.entries[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s is missing", ErrInvalidTransfer, name)
	}
	return readZipEntry(file)
}

/*
ImportOfferingArchive creates a new offering from a previously exported archive.
The caller owns the new class. Exercise dates are shifted by the difference
between the exported end date and the new one, so a course reused in the next
semester keeps its relative schedule instead of importing deadlines in the past.
*/
func ImportOfferingArchive(
	ctx context.Context, data []byte, options OfferingImportOptions,
	claims map[string]any,
) (*models.Offering, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	archive, err := ParseOfferingArchive(data)
	if err != nil {
		return nil, err
	}
	manifest := archive.manifest

	oldEnd, err := time.Parse(time.RFC3339, manifest.Offering.EndDate)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid offering end date", ErrInvalidTransfer)
	}

	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = manifest.Offering.Name
	}
	if err := validation.ValidateRequiredString(
		name, MaxOfferingNameLength,
	); err != nil {
		return nil, err
	}

	description := manifest.Offering.Description
	if options.Description != nil {
		description = *options.Description
	}

	newEnd := oldEnd
	if options.EndDate != nil {
		newEnd = *options.EndDate
	}
	if !newEnd.After(time.Now()) {
		// Importing an archive of a finished class must produce a usable class,
		// not one that is born closed. Six months is a semester.
		newEnd = time.Now().AddDate(0, 6, 0)
	}
	shift := newEnd.Sub(oldEnd)

	allowedTypes, err := loadAllowedFileTypeIDsByExtension(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error starting the offering import transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	enrollmentCode, err := newEnrollmentCode()
	if err != nil {
		return nil, ErrServer
	}

	var offeringID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO offerings
		    (name, owner_id, end_date, description, enrollment_code, visible_to_enroll)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		name, userID, newEnd, description, enrollmentCode,
		manifest.Offering.VisibleToEnroll,
	).Scan(&offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error inserting the imported offering",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	uploaded := make([]string, 0, 16)
	uploadCase := func(key string, content []byte, contentType string) error {
		if err := storage.PutCaseObject(
			ctx, key, bytes.NewReader(content), int64(len(content)), contentType,
		); err != nil {
			return err
		}
		uploaded = append(uploaded, "case:"+key)
		return nil
	}
	uploadFile := func(key string, content []byte) error {
		if err := storage.PutFileObject(
			ctx, key, bytes.NewReader(content), int64(len(content)),
			"application/octet-stream",
		); err != nil {
			return err
		}
		uploaded = append(uploaded, "file:"+key)
		return nil
	}
	fail := func(err error) (*models.Offering, error) {
		deleteImportedObjectsBestEffort(ctx, uploaded)
		return nil, err
	}

	for exerciseIndex, exercise := range manifest.Exercises {
		openDate := exercise.OpenDate.Add(shift)
		deadline := exercise.Deadline.Add(shift)

		var exerciseID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO exercises
			    (offering_id, creator_id, title, description, deadline,
			     open_date, show_before_open_date, removed)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id`,
			offeringID, userID, exercise.Title, exercise.Description,
			deadline, openDate, exercise.ShowBeforeOpenDate, exercise.Removed,
		).Scan(&exerciseID)
		if err != nil {
			slog.ErrorContext(ctx, "error inserting an imported exercise",
				slog.String("error", err.Error()),
			)
			return fail(ErrServer)
		}

		for _, extension := range exercise.AllowedFileTypeExtensions {
			typeID, ok := allowedTypes[strings.ToLower(extension)]
			if !ok {
				return fail(fmt.Errorf(
					"%w: file type %q is not registered; ask an admin to register it before importing",
					ErrImportFailed, extension,
				))
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO exercises_allowed_file_types
				    (exercise_id, allowed_file_type_id)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, exerciseID, typeID,
			); err != nil {
				return fail(ErrServer)
			}
		}

		for caseIndex, testCase := range exercise.TestCases {
			base := fmt.Sprintf(
				"exercises/%03d/testcases/%03d", exerciseIndex+1, caseIndex+1,
			)
			input, err := archive.read(base + "/in")
			if err != nil {
				return fail(err)
			}
			output, err := archive.read(base + "/out")
			if err != nil {
				return fail(err)
			}

			var caseID int64
			err = tx.QueryRowContext(ctx, `
				INSERT INTO exercises_test_cases
				    (exercise_id, input, input_type, show_input, expected_output,
				     expected_output_type, show_expected_output, show_user_output,
				     cpu_time_limit_seconds, mem_usage_limit_bytes,
				     stack_limit_bytes, file_size_limit_bytes)
				VALUES ($1, $2, $3::input_output_type_t, $4, $5,
				        $6::input_output_type_t, $7, $8, $9, $10, $11, $12)
				RETURNING id`,
				exerciseID, testCase.Input, testCase.InputType, testCase.ShowInput,
				testCase.ExpectedOutput, testCase.ExpectedOutputType,
				testCase.ShowExpectedOutput, testCase.ShowUserOutput,
				testCase.CPUTimeLimitSeconds, testCase.MemUsageLimitBytes,
				testCase.StackLimitBytes, testCase.FileSizeLimitBytes,
			).Scan(&caseID)
			if err != nil {
				slog.ErrorContext(ctx, "error inserting an imported test case",
					slog.String("error", err.Error()),
				)
				return fail(ErrServer)
			}

			if err := uploadCase(storage.CaseInputKey(caseID), input, "text/plain; charset=utf-8"); err != nil {
				return fail(ErrServer)
			}
			if err := uploadCase(storage.CaseOutputKey(caseID), output, "text/plain; charset=utf-8"); err != nil {
				return fail(ErrServer)
			}

			for _, fileName := range testCase.Files {
				content, err := archive.read(base + "/files/" + fileName)
				if err != nil {
					return fail(err)
				}
				key := storage.CaseFileKey(caseID, fileName)
				if err := uploadCase(key, content, "application/octet-stream"); err != nil {
					return fail(ErrServer)
				}
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO exercises_test_cases_files
					    (exercise_test_case_id, path)
					VALUES ($1, $2)`, caseID, fileName,
				); err != nil {
					return fail(ErrServer)
				}
			}
		}

		for _, fileName := range exercise.CompilationFiles {
			content, err := archive.read(fmt.Sprintf(
				"exercises/%03d/compilation/%s", exerciseIndex+1, fileName,
			))
			if err != nil {
				return fail(err)
			}
			key := storage.CompilationFileKey(exerciseID, fileName)
			if err := uploadFile(key, content); err != nil {
				return fail(ErrServer)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO exercises_compilation_files (exercise_id, path)
				VALUES ($1, $2)`, exerciseID, fileName,
			); err != nil {
				return fail(ErrServer)
			}
		}

		for _, fileName := range exercise.AttachedFiles {
			content, err := archive.read(fmt.Sprintf(
				"exercises/%03d/attached/%s", exerciseIndex+1, fileName,
			))
			if err != nil {
				return fail(err)
			}
			key := storage.AttachmentKey(exerciseID, fileName)
			if err := uploadFile(key, content); err != nil {
				return fail(ErrServer)
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO exercises_attached_files (exercise_id, path)
				VALUES ($1, $2)`, exerciseID, fileName,
			); err != nil {
				return fail(ErrServer)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing the offering import",
			slog.String("error", err.Error()),
		)
		deleteImportedObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.OfferingKey(offeringID),
		cache.OfferingExercisesKey(offeringID))

	endDate := newEnd.Format(time.RFC3339)
	slog.InfoContext(ctx, "offering imported",
		slog.Int64("offering_id", offeringID),
		slog.Int64("owner_id", userID),
		slog.Int("exercises", len(manifest.Exercises)),
	)

	return &models.Offering{
		ID:              offeringID,
		Name:            name,
		EndDate:         endDate,
		Description:     description,
		EnrollmentCode:  enrollmentCode,
		VisibleToEnroll: manifest.Offering.VisibleToEnroll,
	}, nil
}

func loadAllowedFileTypeIDsByExtension(
	ctx context.Context,
) (map[string]int64, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, extension FROM allowed_file_types`)
	if err != nil {
		slog.ErrorContext(ctx, "error loading allowed file types for import",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	types := make(map[string]int64, 32)
	for rows.Next() {
		var (
			id        int64
			extension string
		)
		if err := rows.Scan(&id, &extension); err != nil {
			return nil, ErrServer
		}
		types[strings.ToLower(extension)] = id
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return types, nil
}

func deleteImportedObjectsBestEffort(ctx context.Context, keys []string) {
	var caseKeys, fileKeys []string
	for _, tagged := range keys {
		switch {
		case strings.HasPrefix(tagged, "case:"):
			caseKeys = append(caseKeys, strings.TrimPrefix(tagged, "case:"))
		case strings.HasPrefix(tagged, "file:"):
			fileKeys = append(fileKeys, strings.TrimPrefix(tagged, "file:"))
		}
	}

	if err := storage.DeleteCaseObjects(ctx, caseKeys); err != nil {
		slog.ErrorContext(ctx, "failed to clean up imported case objects",
			slog.String("error", err.Error()),
		)
	}
	if err := storage.DeleteFileObjects(ctx, fileKeys); err != nil {
		slog.ErrorContext(ctx, "failed to clean up imported file objects",
			slog.String("error", err.Error()),
		)
	}
}
