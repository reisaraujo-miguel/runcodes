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
BatchImportError carries every case of the archive that could not be parsed, so
one upload lists all structural problems instead of one per attempt.
*/
type BatchImportError struct {
	Issues []models.BatchTestCaseIssue
}

func (e *BatchImportError) Error() string {
	if len(e.Issues) == 0 {
		return ErrInvalidTestCase.Error()
	}
	return fmt.Sprintf(
		"%s: %d caso(s) do arquivo não seguem o formato esperado",
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
contains. Nothing is written before every case passes, so a malformed archive
does not leave a half-imported exercise behind.

A case is a pair of `.in` and `.out` files. The pair may sit directly at the root
of the archive, named after the case, or inside a folder of that name:

	1.in
	1.out
	2/
	  2.in
	  2.out
	  ajuda.txt

At the root the pair is matched by base name (`1.in` with `1.out`), so a folder
is only needed when the case carries extra files. Any other file at the root is
ignored and returned in the second value, so the caller can tell the professor
what the import skipped. Inside a case folder every other file is an extra case
file, and a `files/` subfolder holds extras whose names end in `.in` or `.out`.

When the archive is not valid as written and every entry lives inside a single
top-level folder, that folder is treated as the root and the archive is parsed
again. This is what happens when someone zips the folder holding the cases
instead of selecting the files inside it. The second attempt only wins if it
produces cases, so a single case folder keeps its extras and its error messages.

The second return value is the sorted list of ignored root files; it is empty
when every root file was used.
*/
func ParseBatchTestCaseArchive(data []byte) ([]batchTestCase, []string, error) {
	entries, rootFolder, err := readBatchArchive(data, false)
	if err != nil {
		return nil, nil, err
	}

	cases, err := entries.testCases()
	if err == nil {
		return cases, sortedIgnored(entries), nil
	}

	// Retry when the whole archive sits inside one top-level folder. This is
	// what happens when someone zips the folder holding the cases instead of
	// selecting the files inside it.
	if rootFolder == "" {
		return nil, nil, err
	}

	unwrapped, _, readErr := readBatchArchive(data, true)
	if readErr != nil {
		return nil, nil, err
	}
	cases, unwrapErr := unwrapped.testCases()
	if unwrapErr != nil {
		return nil, nil, err
	}
	return cases, sortedIgnored(unwrapped), nil
}

// sortedIgnored returns the archive's ignored root entries in a stable order.
func sortedIgnored(entries *batchArchiveEntries) []string {
	ignored := append([]string(nil), entries.ignored...)
	sort.Strings(ignored)
	return ignored
}

/*
batchArchiveRootFolder returns the only top-level folder of the archive, or ""
when the archive has entries at its root or more than one top-level folder.

An archive whose root already holds a case file cannot be wrapped: treating its
folder as the root would mix both levels. Files the importer ignores anyway, such
as a README beside the folder, do not prevent the folder from being detected.
*/
func batchArchiveRootFolder(files []*zip.File) string {
	folder := ""
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		segments, ok := normalizeArchivePath(file.Name)
		if !ok || ignoredArchiveEntry(segments) {
			continue
		}
		if len(segments) == 1 {
			if caseFileExtension(segments[0]) != "" {
				return ""
			}
			continue
		}
		if folder == "" {
			folder = segments[0]
			continue
		}
		if segments[0] != folder {
			return ""
		}
	}
	return folder
}

// batchArchiveEntries is the raw content of an archive, grouped before it is
// turned into test cases.
type batchArchiveEntries struct {
	// folders holds each case folder's own files.
	folders map[string]map[string][]byte
	// extraFolders holds the files of a case's files/ subfolder.
	extraFolders map[string]map[string][]byte
	// rootFiles holds the root entries, keyed by their full name.
	rootFiles map[string][]byte
	// ignored holds the root entries that are neither .in nor .out.
	ignored []string
	issues  *issueCollector
}

/*
readBatchArchive validates the zip container and groups its entries. Only the
structure is checked here; pairing the files is testCases' job. When unwrapRoot
is set, a single top-level folder that wraps the whole archive is dropped first.
The second result is that folder, empty when the archive has none.
*/
func readBatchArchive(
	data []byte, unwrapRoot bool,
) (*batchArchiveEntries, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("%w: the archive is empty", ErrInvalidArchive)
	}
	if len(data) > MaxBatchArchiveBytes {
		return nil, "", ErrArchiveTooLarge
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	if len(reader.File) > MaxBatchEntries {
		return nil, "", fmt.Errorf("%w: too many entries", ErrArchiveTooLarge)
	}

	rootFolder := batchArchiveRootFolder(reader.File)

	entries := &batchArchiveEntries{
		folders:      make(map[string]map[string][]byte),
		extraFolders: make(map[string]map[string][]byte),
		rootFiles:    make(map[string][]byte),
		issues:       newIssueCollector(),
	}

	var expanded int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		segments, ok := normalizeArchivePath(file.Name)
		if !ok {
			entries.issues.add("", fmt.Sprintf(
				"caminho inválido no arquivo: %q", file.Name,
			))
			continue
		}
		if ignoredArchiveEntry(segments) {
			continue
		}
		if unwrapRoot && rootFolder != "" && len(segments) > 1 &&
			segments[0] == rootFolder {
			segments = segments[1:]
		}

		// Count every entry, including the ignored ones, so the expanded size
		// stays a real bound on what the archive could unpack to.
		expanded += int64(file.UncompressedSize64)
		if expanded > MaxBatchExpandedBytes {
			return nil, "", ErrArchiveTooLarge
		}

		switch {
		case len(segments) == 1:
			if err := entries.addRootEntry(file, segments[0]); err != nil {
				return nil, "", err
			}
		case len(segments) == 2:
			content, err := readZipEntry(file)
			if err != nil {
				return nil, "", err
			}
			addArchiveFile(entries.folders, segments[0], segments[1], content)
		case len(segments) == 3 && segments[1] == "files":
			content, err := readZipEntry(file)
			if err != nil {
				return nil, "", err
			}
			addArchiveFile(entries.extraFolders, segments[0], segments[2], content)
		default:
			entries.issues.add(segments[0], fmt.Sprintf(
				"estrutura de pastas não suportada em %q; os arquivos do caso devem ficar diretamente dentro da pasta do caso",
				file.Name,
			))
		}
	}

	return entries, rootFolder, nil
}

/*
addRootEntry records one file at the root of the archive: a `.in` or `.out` file
belongs to a case, and anything else is ignored and reported to the professor.
*/
func (e *batchArchiveEntries) addRootEntry(file *zip.File, name string) error {
	if caseFileExtension(name) == "" {
		e.ignored = append(e.ignored, name)
		return nil
	}
	if caseBaseName(name) == "" {
		e.issues.add(name, "arquivo de caso sem nome antes da extensão")
		return nil
	}

	content, err := readZipEntry(file)
	if err != nil {
		return err
	}
	e.rootFiles[name] = content
	return nil
}

/*
testCases turns the grouped entries into cases, one per name. A case name comes
from the folder name or, for a root pair, from the file base name. All problems
are collected so the caller can show every offending case at once.
*/
func (e *batchArchiveEntries) testCases() ([]batchTestCase, error) {
	rootInputs, rootOutputs := e.rootPairNames()

	// A files/ subfolder only makes sense inside a case folder; when the pair is
	// at the root there is no folder to hold its extras, and dropping them
	// silently would hide files the professor meant to attach.
	for name := range e.extraFolders {
		if _, isFolder := e.folders[name]; !isFolder {
			e.issues.add(name,
				"os arquivos de files/ precisam ficar dentro da pasta do caso",
			)
		}
	}

	names := make([]string, 0, len(e.folders)+len(rootInputs)+len(rootOutputs))
	for name := range e.folders {
		names = append(names, name)
	}
	for name := range rootInputs {
		names = append(names, name)
	}
	for name := range rootOutputs {
		names = append(names, name)
	}
	names = uniqueSorted(names)

	// Structure problems found while walking the archive come first: they explain
	// why the archive has no usable case.
	if err := e.issues.err(); err != nil {
		return nil, err
	}

	if len(names) == 0 {
		reason := "nenhum caso de teste encontrado (nem par .in/.out na raiz, nem pasta de caso)"
		if len(e.ignored) > 0 {
			sort.Strings(e.ignored)
			reason += fmt.Sprintf(
				"; arquivos ignorados na raiz: %s", limitedList(e.ignored, 10),
			)
		}
		return nil, &BatchImportError{Issues: []models.BatchTestCaseIssue{{
			Folder: "(raiz do arquivo)",
			Reason: reason,
		}}}
	}
	if len(names) > MaxBatchTestCases {
		return nil, fmt.Errorf("%w: at most %d test cases per archive",
			ErrArchiveTooLarge, MaxBatchTestCases)
	}

	cases := make([]batchTestCase, 0, len(names))
	for _, name := range names {
		folder, isFolder := e.folders[name]
		inputName, hasInput := rootInputs[name]
		outputName, hasOutput := rootOutputs[name]

		switch {
		case isFolder && (hasInput || hasOutput):
			e.issues.add(name,
				"o caso aparece duas vezes: como pasta e como par .in/.out na raiz",
			)
		case isFolder:
			if testCase, ok := e.folderCase(name, folder); ok {
				cases = append(cases, testCase)
			}
		default:
			if !hasInput {
				e.issues.add(name, fmt.Sprintf(
					"falta o arquivo %q (entrada do caso)", name+".in",
				))
			}
			if !hasOutput {
				e.issues.add(name, fmt.Sprintf(
					"falta o arquivo %q (saída esperada do caso)", name+".out",
				))
			}
			if hasInput && hasOutput {
				cases = append(cases, batchTestCase{
					Folder:     name,
					InputName:  inputName,
					InputData:  e.rootFiles[inputName],
					OutputName: outputName,
					OutputData: e.rootFiles[outputName],
				})
			}
		}
	}

	return cases, e.issues.err()
}

// rootPairNames matches the root `.in` files with the `.out` files of the same
// base name, reporting a base that declares two inputs or two outputs.
func (e *batchArchiveEntries) rootPairNames() (map[string]string, map[string]string) {
	names := make([]string, 0, len(e.rootFiles))
	for name := range e.rootFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	inputs := make(map[string]string, len(names))
	outputs := make(map[string]string, len(names))
	for _, name := range names {
		base := caseBaseName(name)
		extension := caseFileExtension(name)
		target := inputs
		if extension == ".out" {
			target = outputs
		}
		if existing, duplicate := target[base]; duplicate {
			e.issues.add(base, fmt.Sprintf(
				"mais de um arquivo %s para o mesmo caso: %s, %s",
				extension, existing, name,
			))
			continue
		}
		target[base] = name
	}
	return inputs, outputs
}

// folderCase builds one case from a folder, checking that it declares exactly
// one input and one output.
func (e *batchArchiveEntries) folderCase(
	name string, files map[string][]byte,
) (batchTestCase, bool) {
	var inputNames, outputNames []string
	for fileName := range files {
		switch {
		case hasExtension(fileName, ".in"):
			inputNames = append(inputNames, fileName)
		case hasExtension(fileName, ".out"):
			outputNames = append(outputNames, fileName)
		}
	}
	sort.Strings(inputNames)
	sort.Strings(outputNames)

	broken := false
	switch len(inputNames) {
	case 0:
		e.issues.add(name, "falta um arquivo com extensão .in (entrada do caso)")
		broken = true
	case 1:
		// ok
	default:
		e.issues.add(name, fmt.Sprintf(
			"mais de um arquivo .in na pasta: %s",
			strings.Join(inputNames, ", "),
		))
		broken = true
	}
	switch len(outputNames) {
	case 0:
		e.issues.add(name, "falta um arquivo com extensão .out (saída esperada do caso)")
		broken = true
	case 1:
		// ok
	default:
		e.issues.add(name, fmt.Sprintf(
			"mais de um arquivo .out na pasta: %s",
			strings.Join(outputNames, ", "),
		))
		broken = true
	}
	if broken {
		return batchTestCase{}, false
	}

	inputName := inputNames[0]
	outputName := outputNames[0]

	extras := make([]batchExtraFile, 0, len(files)-2)
	seen := make(map[string]struct{}, len(files))
	for fileName, content := range files {
		seen[fileName] = struct{}{}
		if fileName == inputName || fileName == outputName {
			continue
		}
		extras = append(extras, batchExtraFile{Name: fileName, Data: content})
	}
	for fileName, content := range e.extraFolders[name] {
		if _, duplicate := seen[fileName]; duplicate {
			e.issues.add(name, fmt.Sprintf(
				"arquivo extra duplicado entre a pasta do caso e files/: %s", fileName,
			))
			continue
		}
		seen[fileName] = struct{}{}
		extras = append(extras, batchExtraFile{Name: fileName, Data: content})
	}
	sort.Slice(extras, func(i, j int) bool {
		return extras[i].Name < extras[j].Name
	})

	return batchTestCase{
		Folder:     name,
		InputName:  inputName,
		InputData:  files[inputName],
		OutputName: outputName,
		OutputData: files[outputName],
		ExtraFiles: extras,
	}, true
}

// caseFileExtension is ".in" or ".out" for a case file, ignoring the case of the
// extension, and "" for anything else.
func caseFileExtension(name string) string {
	switch {
	case hasExtension(name, ".in"):
		return ".in"
	case hasExtension(name, ".out"):
		return ".out"
	default:
		return ""
	}
}

// caseBaseName is the case name encoded by a root file: its name without the
// .in/.out extension.
func caseBaseName(name string) string {
	extension := caseFileExtension(name)
	if extension == "" {
		return ""
	}
	return strings.TrimSpace(name[:len(name)-len(extension)])
}

// uniqueSorted returns the names without duplicates, in order.
func uniqueSorted(names []string) []string {
	sort.Strings(names)
	out := names[:0]
	for _, name := range names {
		if len(out) > 0 && out[len(out)-1] == name {
			continue
		}
		out = append(out, name)
	}
	return out
}

// limitedList joins names for a message, capping how many it spells out.
func limitedList(names []string, max int) string {
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, … (+%d)", strings.Join(names[:max], ", "), len(names)-max)
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

	cases, ignored, err := ParseBatchTestCaseArchive(archive)
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
		Ignored:   ignored,
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
