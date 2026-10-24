package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
)

const (
	// MaxBatchArchiveBytes bounds the compressed archive a professor may upload.
	MaxBatchArchiveBytes = 64 << 20
	// MaxBatchExpandedBytes bounds the sum of the files inside the archive, so a
	// zip bomb cannot exhaust memory while every case is validated.
	MaxBatchExpandedBytes = 256 << 20
	// MaxBatchEntries bounds the number of zip entries read.
	MaxBatchEntries = 4000
	// MaxBatchTestCases bounds how many cases one archive may declare.
	MaxBatchTestCases = 500
	// MaxBatchCaseFileBytes bounds one file inside the archive.
	MaxBatchCaseFileBytes = 32 << 20
)

// BatchTestCaseConfig is the configuration applied to every case of a batch.
// The per-case input and output files come from the archive; everything else is
// uniform by design, which is what makes a bulk upload practical.
type BatchTestCaseConfig struct {
	InputType          string
	ExpectedOutputType string

	ShowInput          bool
	ShowExpectedOutput bool
	ShowUserOutput     bool

	CPUTimeLimitSeconds int
	MemUsageLimitBytes  int64
	StackLimitBytes     int64
	FileSizeLimitBytes  int64
}

/*
BatchImportError carries every folder of the archive that could not be parsed,
so one upload lists all structural problems instead of one per attempt.
*/
type BatchImportError struct {
	Issues []models.BatchTestCaseIssue
}

func (e *BatchImportError) Error() string {
	if len(e.Issues) == 0 {
		return ErrInvalidTestCase.Error()
	}
	return fmt.Sprintf(
		"%s: %d folder(s) do arquivo não seguem o formato esperado",
		ErrInvalidTestCase.Error(), len(e.Issues),
	)
}

// Unwrap lets callers treat a structural failure as ErrInvalidTestCase while
// still reading the per-folder details with errors.As.
func (e *BatchImportError) Unwrap() error { return ErrInvalidTestCase }

// batchTestCase is one parsed case folder, still in memory.
type batchTestCase struct {
	Folder     string
	InputName  string
	InputData  []byte
	OutputName string
	OutputData []byte
	ExtraFiles []batchExtraFile
}

type batchExtraFile struct {
	Name string
	Data []byte
}

// issueCollector accumulates one reason list per folder, in first-seen order.
type issueCollector struct {
	order   []string
	reasons map[string][]string
}

func newIssueCollector() *issueCollector {
	return &issueCollector{reasons: make(map[string][]string)}
}

func (c *issueCollector) add(folder, reason string) {
	if folder == "" {
		folder = "(raiz do arquivo)"
	}
	if _, ok := c.reasons[folder]; !ok {
		c.order = append(c.order, folder)
	}
	// The same problem can be reached through duplicate entries; keep the list
	// readable by not repeating an identical reason.
	for _, existing := range c.reasons[folder] {
		if existing == reason {
			return
		}
	}
	c.reasons[folder] = append(c.reasons[folder], reason)
}

func (c *issueCollector) err() error {
	if len(c.order) == 0 {
		return nil
	}

	issues := make([]models.BatchTestCaseIssue, 0, len(c.order))
	for _, folder := range c.order {
		issues = append(issues, models.BatchTestCaseIssue{
			Folder: folder,
			Reason: strings.Join(c.reasons[folder], "; "),
		})
	}
	return &BatchImportError{Issues: issues}
}

/*
normalizeArchivePath turns a zip entry name into slash-separated segments and
rejects absolute paths, parent traversal and empty names. It is the boundary that
keeps an archive from addressing anything outside the in-memory file map.
*/
func normalizeArchivePath(name string) ([]string, bool) {
	value := strings.ReplaceAll(name, "\\", "/")
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "./")
	if value == "" || strings.HasPrefix(value, "/") {
		return nil, false
	}

	segments := strings.Split(value, "/")
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." || strings.ContainsRune(segment, '\x00') {
			return nil, false
		}
		cleaned = append(cleaned, segment)
	}
	if len(cleaned) == 0 {
		return nil, false
	}
	return cleaned, true
}

// ignoredArchiveEntry reports the metadata macOS adds to every zip, which is not
// part of a test case and should not make an otherwise valid archive fail.
func ignoredArchiveEntry(segments []string) bool {
	if len(segments) == 0 {
		return true
	}
	if segments[0] == "__MACOSX" {
		return true
	}
	base := segments[len(segments)-1]
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

func hasExtension(name, extension string) bool {
	return strings.HasSuffix(strings.ToLower(name), extension)
}

/*
ParseBatchTestCaseArchive validates the whole archive and returns the cases it
contains. Nothing is written before every folder passes, so a malformed folder
does not leave a half-imported exercise behind.

The accepted layout is one folder per test case:

	1/input.in
	1/output.out
	1/extra.txt
	2/input.in
	2/output.out
	2/files/notes.in   (an extra file that would otherwise look like the input)

Each case folder must contain exactly one `.in` and exactly one `.out` file. Any
other file is an extra test-case file, materialised next to the case input by the
judge. The `files/` subfolder is accepted so an exported archive can carry an
extra file whose name ends in `.in` or `.out` without ambiguity.
*/
func ParseBatchTestCaseArchive(data []byte) ([]batchTestCase, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: the archive is empty", ErrInvalidArchive)
	}
	if len(data) > MaxBatchArchiveBytes {
		return nil, ErrArchiveTooLarge
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(reader.File) > MaxBatchEntries {
		return nil, fmt.Errorf("%w: too many entries", ErrArchiveTooLarge)
	}

	folders := make(map[string]map[string][]byte)
	extraFolders := make(map[string]map[string][]byte)
	issues := newIssueCollector()

	var expanded int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		segments, ok := normalizeArchivePath(file.Name)
		if !ok {
			issues.add("", fmt.Sprintf(
				"caminho inválido no arquivo: %q", file.Name,
			))
			continue
		}
		if ignoredArchiveEntry(segments) {
			continue
		}

		switch {
		case len(segments) == 1:
			issues.add(segments[0],
				"arquivo solto na raiz; cada caso deve ficar em uma pasta própria",
			)
			continue
		case len(segments) == 2:
			name := segments[1]
			expanded += int64(file.UncompressedSize64)
			if expanded > MaxBatchExpandedBytes {
				return nil, ErrArchiveTooLarge
			}
			content, err := readZipEntry(file)
			if err != nil {
				return nil, err
			}
			addArchiveFile(folders, segments[0], name, content)
			continue
		case len(segments) == 3 && segments[1] == "files":
			name := segments[2]
			expanded += int64(file.UncompressedSize64)
			if expanded > MaxBatchExpandedBytes {
				return nil, ErrArchiveTooLarge
			}
			content, err := readZipEntry(file)
			if err != nil {
				return nil, err
			}
			addArchiveFile(extraFolders, segments[0], name, content)
			continue
		default:
			issues.add(segments[0], fmt.Sprintf(
				"estrutura de pastas não suportada em %q; os arquivos do caso devem ficar diretamente dentro da pasta do caso",
				file.Name,
			))
			continue
		}
	}

	if err := issues.err(); err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		return nil, &BatchImportError{Issues: []models.BatchTestCaseIssue{{
			Folder: "(raiz do arquivo)",
			Reason: "nenhuma pasta de caso de teste foi encontrada",
		}}}
	}
	if len(folders) > MaxBatchTestCases {
		return nil, fmt.Errorf("%w: at most %d test cases per archive",
			ErrArchiveTooLarge, MaxBatchTestCases)
	}

	names := make([]string, 0, len(folders))
	for folder := range folders {
		names = append(names, folder)
	}
	sort.Strings(names)

	cases := make([]batchTestCase, 0, len(names))
	for _, folder := range names {
		files := folders[folder]

		var inputNames, outputNames []string
		for name := range files {
			switch {
			case hasExtension(name, ".in"):
				inputNames = append(inputNames, name)
			case hasExtension(name, ".out"):
				outputNames = append(outputNames, name)
			}
		}
		sort.Strings(inputNames)
		sort.Strings(outputNames)

		switch len(inputNames) {
		case 0:
			issues.add(folder, "falta um arquivo com extensão .in (entrada do caso)")
		case 1:
			// ok
		default:
			issues.add(folder, fmt.Sprintf(
				"mais de um arquivo .in na pasta: %s",
				strings.Join(inputNames, ", "),
			))
		}
		switch len(outputNames) {
		case 0:
			issues.add(folder, "falta um arquivo com extensão .out (saída esperada do caso)")
		case 1:
			// ok
		default:
			issues.add(folder, fmt.Sprintf(
				"mais de um arquivo .out na pasta: %s",
				strings.Join(outputNames, ", "),
			))
		}
		if err := issues.err(); err != nil {
			continue
		}

		inputName := inputNames[0]
		outputName := outputNames[0]

		extras := make([]batchExtraFile, 0, len(files)-2)
		seen := make(map[string]struct{}, len(files))
		for name, content := range files {
			if _, duplicate := seen[name]; duplicate {
				issues.add(folder, fmt.Sprintf("arquivo duplicado na pasta: %s", name))
				continue
			}
			seen[name] = struct{}{}
			if name == inputName || name == outputName {
				continue
			}
			extras = append(extras, batchExtraFile{Name: name, Data: content})
		}
		for name, content := range extraFolders[folder] {
			if _, duplicate := seen[name]; duplicate {
				issues.add(folder, fmt.Sprintf(
					"arquivo extra duplicado entre a pasta do caso e files/: %s", name,
				))
				continue
			}
			seen[name] = struct{}{}
			extras = append(extras, batchExtraFile{Name: name, Data: content})
		}
		sort.Slice(extras, func(i, j int) bool {
			return extras[i].Name < extras[j].Name
		})

		cases = append(cases, batchTestCase{
			Folder:     folder,
			InputName:  inputName,
			InputData:  files[inputName],
			OutputName: outputName,
			OutputData: files[outputName],
			ExtraFiles: extras,
		})
	}

	if err := issues.err(); err != nil {
		return nil, err
	}

	return cases, nil
}

func addArchiveFile(
	target map[string]map[string][]byte, folder, name string, content []byte,
) {
	if target[folder] == nil {
		target[folder] = make(map[string][]byte)
	}
	target[folder][name] = content
}

func readZipEntry(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > MaxBatchCaseFileBytes {
		return nil, ErrArchiveTooLarge
	}

	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %q", ErrInvalidArchive, file.Name)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, MaxBatchCaseFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %q", ErrInvalidArchive, file.Name)
	}
	if len(data) > MaxBatchCaseFileBytes {
		return nil, ErrArchiveTooLarge
	}
	return data, nil
}

/*
CreateTestCasesBatch validates and stores every case of an archive in one
transaction. The per-case configuration is applied uniformly, as the professor
set it once for the batch.
*/
func CreateTestCasesBatch(
	ctx context.Context, exerciseID int64, archive []byte,
	config BatchTestCaseConfig, claims map[string]any,
) (*models.BatchTestCaseImportResponse, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	authoringID := acc.authoringExerciseID()

	if err := validateBatchConfig(&config); err != nil {
		return nil, err
	}

	cases, err := ParseBatchTestCaseArchive(archive)
	if err != nil {
		return nil, err
	}

	// A text case is stored in a text column and materialised by the judge as a
	// file, so invalid UTF-8 would be rejected by PostgreSQL halfway through the
	// batch. Reject it before the first insert, naming the folder.
	issues := newIssueCollector()
	for _, testCase := range cases {
		if config.InputType == IOTypeText && !utf8.Valid(testCase.InputData) {
			issues.add(testCase.Folder,
				"o arquivo .in não é texto UTF-8 válido para o tipo de entrada escolhido",
			)
		}
		if config.ExpectedOutputType == IOTypeText && !utf8.Valid(testCase.OutputData) {
			issues.add(testCase.Folder,
				"o arquivo .out não é texto UTF-8 válido para o tipo de saída escolhido",
			)
		}
	}
	if err := issues.err(); err != nil {
		return nil, err
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error starting the batch transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	uploaded := make([]string, 0, len(cases)*2)
	created := make([]models.TestCase, 0, len(cases))

	upload := func(key string, content []byte, contentType string) error {
		if err := storage.PutCaseObject(
			ctx, key, bytes.NewReader(content), int64(len(content)), contentType,
		); err != nil {
			return err
		}
		uploaded = append(uploaded, key)
		return nil
	}

	for _, testCase := range cases {
		inputColumn := testCase.InputName
		if config.InputType == IOTypeText {
			inputColumn = string(testCase.InputData)
		}
		outputColumn := testCase.OutputName
		if config.ExpectedOutputType == IOTypeText {
			outputColumn = string(testCase.OutputData)
		}

		var caseID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO exercises_test_cases
			    (exercise_id, input, input_type, show_input, expected_output,
			     expected_output_type, show_expected_output, show_user_output,
			     cpu_time_limit_seconds, mem_usage_limit_bytes,
			     stack_limit_bytes, file_size_limit_bytes)
			VALUES ($1, $2, $3::input_output_type_t, $4, $5,
			        $6::input_output_type_t, $7, $8, $9, $10, $11, $12)
			RETURNING id`,
			authoringID, inputColumn, config.InputType, config.ShowInput,
			outputColumn, config.ExpectedOutputType, config.ShowExpectedOutput,
			config.ShowUserOutput, config.CPUTimeLimitSeconds,
			config.MemUsageLimitBytes, config.StackLimitBytes,
			config.FileSizeLimitBytes,
		).Scan(&caseID)
		if err != nil {
			slog.ErrorContext(ctx, "error inserting a batch test case",
				slog.Int64("exercise_id", authoringID),
				slog.String("folder", testCase.Folder),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}

		inputContentType := "application/octet-stream"
		if config.InputType == IOTypeText {
			inputContentType = "text/plain; charset=utf-8"
		}
		if err := upload(
			storage.CaseInputKey(caseID), testCase.InputData, inputContentType,
		); err != nil {
			slog.ErrorContext(ctx, "error uploading a batch test case input",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}

		outputContentType := "application/octet-stream"
		if config.ExpectedOutputType == IOTypeText {
			outputContentType = "text/plain; charset=utf-8"
		}
		if err := upload(
			storage.CaseOutputKey(caseID), testCase.OutputData, outputContentType,
		); err != nil {
			slog.ErrorContext(ctx, "error uploading a batch test case output",
				slog.Int64("test_case_id", caseID),
				slog.String("error", err.Error()),
			)
			deleteCaseObjectsBestEffort(ctx, uploaded)
			return nil, ErrServer
		}

		files := make([]models.TestCaseFile, 0, len(testCase.ExtraFiles))
		for _, extra := range testCase.ExtraFiles {
			key := storage.CaseFileKey(caseID, extra.Name)
			if err := upload(key, extra.Data, "application/octet-stream"); err != nil {
				slog.ErrorContext(ctx, "error uploading a batch test case file",
					slog.Int64("test_case_id", caseID),
					slog.String("error", err.Error()),
				)
				deleteCaseObjectsBestEffort(ctx, uploaded)
				return nil, ErrServer
			}

			var fileID int64
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO exercises_test_cases_files
				    (exercise_test_case_id, path)
				VALUES ($1, $2)
				RETURNING id`, caseID, storage.FileBasename(extra.Name),
			).Scan(&fileID); err != nil {
				slog.ErrorContext(ctx, "error inserting a batch test case file",
					slog.Int64("test_case_id", caseID),
					slog.String("error", err.Error()),
				)
				deleteCaseObjectsBestEffort(ctx, uploaded)
				return nil, ErrServer
			}
			files = append(files, models.TestCaseFile{
				ID: fileID, Path: storage.FileBasename(extra.Name),
			})
		}

		created = append(created, models.TestCase{
			ID:                  caseID,
			ExerciseID:          authoringID,
			Input:               inputColumn,
			InputType:           config.InputType,
			ShowInput:           config.ShowInput,
			ExpectedOutput:      outputColumn,
			ExpectedOutputType:  config.ExpectedOutputType,
			ShowExpectedOutput:  config.ShowExpectedOutput,
			ShowUserOutput:      config.ShowUserOutput,
			CPUTimeLimitSeconds: config.CPUTimeLimitSeconds,
			MemUsageLimitBytes:  config.MemUsageLimitBytes,
			StackLimitBytes:     config.StackLimitBytes,
			FileSizeLimitBytes:  config.FileSizeLimitBytes,
			Files:               files,
		})
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing the batch import",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		deleteCaseObjectsBestEffort(ctx, uploaded)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "test cases imported in batch",
		slog.Int64("exercise_id", authoringID),
		slog.Int("created", len(created)),
	)

	return &models.BatchTestCaseImportResponse{
		Created:   len(created),
		TestCases: created,
	}, nil
}

func validateBatchConfig(config *BatchTestCaseConfig) error {
	if config.InputType == "" {
		config.InputType = IOTypeText
	}
	if config.ExpectedOutputType == "" {
		config.ExpectedOutputType = IOTypeText
	}
	if err := validateIOType(config.InputType); err != nil {
		return err
	}
	if err := validateIOType(config.ExpectedOutputType); err != nil {
		return err
	}
	if config.CPUTimeLimitSeconds < 0 ||
		config.MemUsageLimitBytes < 0 ||
		config.StackLimitBytes < 0 ||
		config.FileSizeLimitBytes < 0 {
		return fmt.Errorf("%w: limits cannot be negative", ErrInvalidTestCase)
	}
	return nil
}

/*
ExportExerciseTestCasesArchive renders every test case of an exercise in the
same folder layout the batch importer accepts. The professor can download an
exercise's tests and upload the archive to another exercise, where the same
uniform configuration is applied on import.
*/
func ExportExerciseTestCasesArchive(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (filename string, data []byte, err error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return "", nil, err
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
		slog.ErrorContext(ctx, "error loading test cases for export",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return "", nil, ErrServer
	}

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
			rows.Close()
			slog.ErrorContext(ctx, "error scanning a test case for export",
				slog.String("error", err.Error()),
			)
			return "", nil, ErrServer
		}
		testCase.Files = []models.TestCaseFile{}
		cases = append(cases, testCase)
		ids = append(ids, testCase.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", nil, ErrServer
	}
	rows.Close()

	files, err := loadTestCaseFiles(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	for index := range cases {
		if list, ok := files[cases[index].ID]; ok {
			cases[index].Files = list
		}
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for index, testCase := range cases {
		folder := fmt.Sprintf("caso_%03d", index+1)

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

		if err := writeZipEntry(writer, folder+"/input.in", input); err != nil {
			writer.Close()
			return "", nil, err
		}
		if err := writeZipEntry(writer, folder+"/output.out", output); err != nil {
			writer.Close()
			return "", nil, err
		}

		for _, file := range testCase.Files {
			name := storage.FileBasename(file.Path)
			content, err := readCaseObject(
				ctx, storage.CaseFileKey(testCase.ID, name),
			)
			if err != nil {
				writer.Close()
				return "", nil, err
			}
			entryName := folder + "/" + name
			// An extra file whose name ends in .in/.out would be ambiguous with
			// the case input/output on import. The files/ subfolder marks it as an
			// extra, and the importer reads it back as one.
			if hasExtension(name, ".in") || hasExtension(name, ".out") {
				entryName = folder + "/files/" + name
			}
			if err := writeZipEntry(writer, entryName, content); err != nil {
				writer.Close()
				return "", nil, err
			}
		}
	}

	if err := writer.Close(); err != nil {
		slog.ErrorContext(ctx, "error finishing the test cases archive",
			slog.Int64("exercise_id", authoringID),
			slog.String("error", err.Error()),
		)
		return "", nil, ErrServer
	}

	return fmt.Sprintf("exercicio-%d-casos-de-teste.zip", exerciseID), buffer.Bytes(), nil
}

// loadCaseObject returns the content of a case input/output. A text case has no
// authoritative S3 object in a migrated install, so the text column is the
// fallback; a file case must have its object.
func loadCaseObject(
	ctx context.Context, caseID int64, contentType, columnValue string, output bool,
) ([]byte, error) {
	key := storage.CaseInputKey(caseID)
	if output {
		key = storage.CaseOutputKey(caseID)
	}

	content, err := readCaseObject(ctx, key)
	if err == nil {
		return content, nil
	}
	if contentType == IOTypeText && errors.Is(err, storage.ErrObjectNotFound) {
		return []byte(columnValue), nil
	}
	if errors.Is(err, storage.ErrObjectNotFound) {
		slog.WarnContext(ctx, "test case object is missing for export",
			slog.Int64("test_case_id", caseID),
			slog.String("key", key),
		)
		return nil, ErrServer
	}
	return nil, err
}

func readCaseObject(ctx context.Context, key string) ([]byte, error) {
	object, err := storage.GetCaseObject(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, err
		}
		slog.ErrorContext(ctx, "error reading a test case object",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer object.Body.Close()

	data, err := io.ReadAll(io.LimitReader(object.Body, MaxBatchCaseFileBytes+1))
	if err != nil {
		slog.ErrorContext(ctx, "error reading a test case object body",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	if len(data) > MaxBatchCaseFileBytes {
		return nil, ErrArchiveTooLarge
	}
	return data, nil
}

func writeZipEntry(writer *zip.Writer, name string, content []byte) error {
	entry, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("creating zip entry: %w", err)
	}
	if _, err := entry.Write(content); err != nil {
		return fmt.Errorf("writing zip entry: %w", err)
	}
	return nil
}
