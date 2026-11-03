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

// outputCasePrefix marks the files one test case produced: `test_<case id>/...`.
const outputCasePrefix = "test_"

/*
OutputFileTestCaseID returns the test case a generated file came from. It returns
0 for a file written straight into the runner's output directory
(`outputfiles/...`), which belongs to the submission as a whole rather than to one
case.
*/
func OutputFileTestCaseID(filePath string) int64 {
	prefix, _, found := strings.Cut(filePath, "/")
	if !found || !strings.HasPrefix(prefix, outputCasePrefix) {
		return 0
	}

	caseID, err := strconv.ParseInt(strings.TrimPrefix(prefix, outputCasePrefix), 10, 64)
	if err != nil || caseID <= 0 {
		return 0
	}

	return caseID
}

/*
OutputFileVisible reports whether a generated file may be shown to the student. A
file that belongs to no case is always visible; a case's file follows the case's
`show_user_output`.
*/
func OutputFileVisible(filePath string, hidden map[int64]struct{}) bool {
	caseID := OutputFileTestCaseID(filePath)
	if caseID == 0 {
		return true
	}

	_, isHidden := hidden[caseID]
	return !isHidden
}

/*
VisibleOutputFiles drops the files produced by cases whose output the author did
not release. The order of the listing is preserved.
*/
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
