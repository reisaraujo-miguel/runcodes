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
// `test_v2_<case number>/...`. The number is the case's frozen 1-based position
// in the exercise (commit_test_case_limits.case_number), not its platform-wide
// id. The `v2` segment distinguishes this namespace from the legacy one below.
const outputCasePrefix = "test_v2_"

// legacyOutputCasePrefix marks the files written before the numbering was
// frozen, when the judge named the directory with the platform-wide case id:
// `test_<case id>/...`. Those objects keep their meaning, so the resolver reads
// them as ids instead of misreading the id as a position.
const legacyOutputCasePrefix = "test_"

// caseDirNumber parses `<prefix><positive integer>/<rest>` and returns the
// integer. It returns 0 for a run-level file (no case directory) and for a
// prefix whose segment is not a positive integer.
func caseDirNumber(filePath, prefix string) int64 {
	first, _, found := strings.Cut(filePath, "/")
	if !found || !strings.HasPrefix(first, prefix) {
		return 0
	}

	number, err := strconv.ParseInt(strings.TrimPrefix(first, prefix), 10, 64)
	if err != nil || number <= 0 {
		return 0
	}

	return number
}

/*
OutputFileCaseNumber returns the case number a generated file came from, for a
path in the current `test_v2_<number>/...` form. It returns 0 for a run-level
file (written straight into the runner's output directory) and for a legacy
path, which carries an id instead.
*/
func OutputFileCaseNumber(filePath string) int64 {
	return caseDirNumber(filePath, outputCasePrefix)
}

/*
OutputFileLegacyTestCaseID returns the platform-wide test-case id a generated
file came from, for a path in the legacy `test_<case id>/...` form. It returns 0
for the current form and for a run-level file.
*/
func OutputFileLegacyTestCaseID(filePath string) int64 {
	if OutputFileCaseNumber(filePath) != 0 {
		return 0
	}

	return caseDirNumber(filePath, legacyOutputCasePrefix)
}

/*
ResolveOutputFileTestCaseID maps a generated file's path to the platform-wide
test-case id, using the commit's frozen numbering (CommitCaseIndex). A run-level
file resolves to 0; a file whose case no longer belongs to the commit is
reported as unknown and must not be served.
*/
func ResolveOutputFileTestCaseID(
	filePath string, index CommitCaseIndex,
) (int64, bool) {
	if number := OutputFileCaseNumber(filePath); number != 0 {
		return index.IDForNumber(int(number))
	}

	if id := OutputFileLegacyTestCaseID(filePath); id != 0 {
		if !index.HasID(id) {
			return 0, false
		}
		return id, true
	}

	return 0, true
}

/*
OutputFileVisibleForCases reports whether a generated file may be shown to the
student. A file that belongs to no case is always visible; a case's file follows
the case's `show_user_output`, resolved through the commit's frozen numbering.
*/
func OutputFileVisibleForCases(
	filePath string, hidden map[int64]struct{}, index CommitCaseIndex,
) bool {
	caseID, ok := ResolveOutputFileTestCaseID(filePath, index)
	if !ok {
		return false
	}
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
	files []storage.OutputFile, hidden map[int64]struct{}, index CommitCaseIndex,
) []storage.OutputFile {
	visible := make([]storage.OutputFile, 0, len(files))
	for _, file := range files {
		if OutputFileVisibleForCases(file.Path, hidden, index) {
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
