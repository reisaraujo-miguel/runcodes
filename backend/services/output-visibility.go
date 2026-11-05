package services

import (
	"context"
	"strconv"
	"strings"

	"github.com/runcodes-icmc/runcodes/storage"
)

/*
A generated file belongs to the test case that produced it, and a case whose
program output the author did not release hides its files as well: those files
are often the very output the case compares, so publishing them would leak what
the redaction in the results panel is protecting.
*/

// outputCasePrefix marks the files one test case produced:
// `test_<case number>/...`. The number is the case's 1-based position in the
// exercise, not its platform-wide id.
const outputCasePrefix = "test_"

/*
OutputFileCaseNumber returns the case number a generated file came from. It
returns 0 for a file written straight into the runner's output directory
(`outputfiles/...`), which belongs to the submission as a whole rather than to one
case.
*/
func OutputFileCaseNumber(filePath string) int64 {
	prefix, _, found := strings.Cut(filePath, "/")
	if !found || !strings.HasPrefix(prefix, outputCasePrefix) {
		return 0
	}

	number, err := strconv.ParseInt(strings.TrimPrefix(prefix, outputCasePrefix), 10, 64)
	if err != nil || number <= 0 {
		return 0
	}

	return number
}

// OutputFileTestCaseID is kept for callers that only need the numeric prefix.
// The value is the exercise-local number; use ResolveOutputFileTestCaseID to
// turn it into a database id.
func OutputFileTestCaseID(filePath string) int64 {
	return OutputFileCaseNumber(filePath)
}

/*
ResolveOutputFileTestCaseID maps a generated file's case number to the
platform-wide test-case id. caseIDs is the exercise order returned by
CommitTestCaseIDs. A file outside a case directory resolves to 0; a file whose
number has no case (a deleted case, or a path written by another version) is
reported as unknown and must not be served.
*/
func ResolveOutputFileTestCaseID(filePath string, caseIDs []int64) (int64, bool) {
	number := OutputFileCaseNumber(filePath)
	if number == 0 {
		return 0, true
	}

	index := number - 1
	if index < 0 || index >= int64(len(caseIDs)) {
		return 0, false
	}

	return caseIDs[index], true
}

/*
OutputFileVisibleForCases reports whether a generated file may be shown to the
student. A file that belongs to no case is always visible; a case's file follows
the case's `show_user_output`, resolved through the exercise's case order.
*/
func OutputFileVisibleForCases(
	filePath string, hidden map[int64]struct{}, caseIDs []int64,
) bool {
	caseID, ok := ResolveOutputFileTestCaseID(filePath, caseIDs)
	if !ok {
		return false
	}
	if caseID == 0 {
		return true
	}

	_, isHidden := hidden[caseID]
	return !isHidden
}

// OutputFileVisible is the id-based form kept for tests and callers that
// already hold a resolved case id.
func OutputFileVisible(filePath string, hidden map[int64]struct{}) bool {
	caseID := OutputFileCaseNumber(filePath)
	if caseID == 0 {
		return true
	}

	_, isHidden := hidden[caseID]
	return !isHidden
}

/*
VisibleOutputFilesForCases drops the files produced by cases whose output the
author did not release. The order of the listing is preserved.
*/
func VisibleOutputFilesForCases(
	files []storage.OutputFile, hidden map[int64]struct{}, caseIDs []int64,
) []storage.OutputFile {
	visible := make([]storage.OutputFile, 0, len(files))
	for _, file := range files {
		if OutputFileVisibleForCases(file.Path, hidden, caseIDs) {
			visible = append(visible, file)
		}
	}

	return visible
}

// VisibleOutputFiles is the id-based form kept for compatibility.
func VisibleOutputFiles(
	files []storage.OutputFile, hidden map[int64]struct{},
) []storage.OutputFile {
	if len(hidden) == 0 {
		return files
	}

	visible := make([]storage.OutputFile, 0, len(files))
	for _, file := range files {
		if OutputFileVisible(file.Path, hidden) {
			visible = append(visible, file)
		}
	}

	return visible
}

/*
MergeCaseIDs returns the union of the given sets. It lets the artifact endpoints
build one "do not publish" set from the rules that apply: the author hid the
case's output, or the run was interrupted and left only a partial file.
*/
func MergeCaseIDs(sets ...map[int64]struct{}) map[int64]struct{} {
	merged := make(map[int64]struct{})
	for _, set := range sets {
		for caseID := range set {
			merged[caseID] = struct{}{}
		}
	}

	return merged
}

/*
UnavailableGeneratedFileCases returns the cases whose generated files must not be
served to the student, merging both rules.
*/
func UnavailableGeneratedFileCases(
	ctx context.Context, commitID int64,
) (map[int64]struct{}, error) {
	hidden, err := HiddenTestCaseIDs(ctx, commitID)
	if err != nil {
		return nil, err
	}

	interrupted, err := InterruptedTestCaseIDs(ctx, commitID)
	if err != nil {
		return nil, err
	}

	return MergeCaseIDs(hidden, interrupted), nil
}
