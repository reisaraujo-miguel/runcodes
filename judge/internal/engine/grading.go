package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/runcodes-icmc/judge/internal/cmp"
	"github.com/runcodes-icmc/judge/internal/model"
)

// monitorInfo is the subset of the monitor output the judge cares about.
type monitorInfo struct {
	Signal string
	Time   float64
	Mem    int64
	HasMem bool
}

// maxMonitorBytes bounds the monitor output the judge parses. The file is written
// in the workspace by the submission, so it is read like every other file there:
// as a bounded, symlink-refusing read of a regular file.
const maxMonitorBytes = 1 << 20

// readMonitor parses the monitor's INI-ish output (`<id>.monitor_out`). Keys
// live under an `[info]` section; the parser is deliberately lenient.
//
// The report carries the exit status, the signal and the wall clock time. It
// carries no memory figure: the harness runs the program through `timeout`, so the
// monitor is not its parent and cannot read its peak RSS — it is never measured
// rather than guessed, which is why MemUsage stays -1 (unknown).
func readMonitor(path string) monitorInfo {
	var info monitorInfo

	raw, err := readRegularBounded(path, maxMonitorBytes)
	if err != nil {
		return info // missing file is not an error, just means no info
	}

	// Parse the file line by line, looking for key=value pairs.
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments (lines starting with [ or # or ;).
		if line == "" ||
			strings.HasPrefix(line, "[") ||
			strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, ";") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "signal":
			info.Signal = value
		case "time":
			info.Time, _ = strconv.ParseFloat(value, 64)
		case "mem", "memory", "mem_usage", "memused":
			// Parse memory usage as an integer. If parsing fails, we ignore it.
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				info.Mem = n
				info.HasMem = true
			}
		}
	}

	return info
}

// gradeAll grades every test case, downloading expected outputs from S3.
func (e *Engine) gradeAll(ctx context.Context, commit *model.Commit, ws *workspace) ([]model.CaseResult, error) {
	results := make([]model.CaseResult, 0, len(ws.TestCases))

	// Grade each test case and collect the results.
	for _, tc := range ws.TestCases {
		result, err := e.gradeCase(ctx, tc, ws)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

// gradeCase grades a single test case, reading the user's output and comparing it
// to the expected output. It also reads the monitor output for resource usage.
//
// Every file read from the workspace is checked to be a regular file and bounded:
// the workspace is written by the untrusted submission, so a symlink there would
// otherwise choose what the judge reads and compares.
func (e *Engine) gradeCase(ctx context.Context, tc model.TestCase, ws *workspace) (model.CaseResult, error) {
	result := model.CaseResult{
		TestCaseID: tc.ID,
		MemUsage:   -1,
		OutputType: "text",
	}

	// Read the user's output, error output, and monitor output from the workspace.
	outputPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.output", tc.ID))
	errorPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.error", tc.ID))
	monitorPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.monitor_out", tc.ID))

	// Read the monitor output to get resource usage and signal information.
	info := readMonitor(monitorPath)
	result.CPUTime = info.Time
	if info.HasMem {
		result.MemUsage = info.Mem
	}

	errContent := readLimited(errorPath, e.cfg.MaxOutputFileSize)

	// Read the submission's output up front, under the comparison bound, so both
	// sides of the comparison below are data the judge read itself.
	userOutput, outputErr := readRegularBounded(outputPath, e.cfg.MaxCompareFileBytes)

	switch {
	case outputErr != nil:
		// A missing, oversized or non-regular output file cannot be a correct
		// answer, so the case fails instead of being compared. The detail stays in
		// the log: an os error would name a judge-side path to the student.
		e.logger.Warn("could not read a test case output",
			"test_case_id", tc.ID,
			"error", outputErr,
		)
		result.Status = model.CaseKilledBySignal
		result.ErrorMessage = "the output of this test case could not be read"

	case info.Signal != "" || errContent != "":
		// If the monitor indicates a signal or there is error output, we consider the case killed by a signal.
		result.Status = model.CaseKilledBySignal
		result.StatusMsg = info.Signal
		result.ErrorMessage = errContent

	default:
		// Download the expected output from S3 for comparison, into the directory
		// the container cannot write to, then read it under the comparison bound.
		expectedPath := ws.expectedOutputPath(tc.ID)
		if err := e.s3.FetchCaseOutput(ctx, tc.ID, expectedPath); err != nil {
			return result, fmt.Errorf("download expected output of case %d: %w", tc.ID, err)
		}

		expected, err := readRegularBounded(expectedPath, e.cfg.MaxCompareFileBytes)
		if err != nil {
			return result, fmt.Errorf("read expected output of case %d: %w", tc.ID, err)
		}

		// Compare the user's output with the expected output based on the expected output type.
		result.Status = compareOutput(userOutput, expected, tc.ExpectedOutputType)
	}

	// Read the user's output, limited to the configured maximum size, and store it in the result.
	result.UserOutput = readLimited(outputPath, e.cfg.MaxOutputFileSize)

	return result, nil
}

// compareOutput maps the legacy comparison modes onto the new schema:
// `file` expected outputs are compared byte-for-byte, `text` ones with the
// strict/lenient text comparators.
func compareOutput(userOutput, expected []byte, expectedType string) model.CaseStatus {
	// If the expected output type is "file", we perform a byte-for-byte comparison
	// of the user's output and the expected output.
	if expectedType == "file" {
		if bytes.Equal(userOutput, expected) {
			return model.CaseCorrect
		}

		return model.CaseKilledBySignal
	}

	// For "text" expected outputs, we use the text comparison functions to determine
	// the case status. The comparison is done in a lenient manner, allowing for
	// differences in whitespace and case.
	switch {
	case cmp.TextEqualBytes(userOutput, expected):
		return model.CaseCorrect
	case cmp.TextLenientBytes(userOutput, expected):
		return model.CaseBadFormat
	default:
		return model.CaseKilledBySignal
	}
}

// openRegular opens path for reading only when it is a regular file. The
// workspace is written by the untrusted submission, and os.Open follows symlinks,
// so without this check a submission could point the judge at any file the judge
// process can read (and have its contents published as its own output).
//
// O_NOFOLLOW closes the race between the check and the open, so the guarantee
// holds even if the path is replaced in between.
func openRegular(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(path))
	}

	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// readRegularBounded reads at most max bytes of a regular file. An oversized file
// is an error rather than a truncated comparison, so a submission cannot win or
// lose a case on a prefix of its output.
func readRegularBounded(path string, max int64) ([]byte, error) {
	f, err := openRegular(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}

	if int64(len(raw)) > max {
		return nil, fmt.Errorf("file exceeds the %d byte limit", max)
	}

	return raw, nil
}

// readLimited returns at most max bytes of a regular file, replacing invalid
// UTF-8 so it stays safe to store in a text column. A missing file yields "".
//
// Non-regular files are refused: everything under the workspace is written by the
// submission, and following a symlink there would make the judge read — and
// publish as the submission's own output — a file of the submission's choosing.
func readLimited(path string, max int64) string {
	f, err := openRegular(path)
	if err != nil {
		return ""
	}

	defer f.Close()

	var r io.Reader = f

	// If max is greater than 0, we limit the reader to max bytes. Otherwise, we read the entire file.
	if max > 0 {
		r = io.LimitReader(f, max)
	}

	// Read the content of the file into a byte slice.
	raw, err := io.ReadAll(r)
	if err != nil {
		return ""
	}

	// Convert the raw bytes to a valid UTF-8 string, replacing invalid sequences with the Unicode replacement character.
	return strings.ToValidUTF8(string(raw), "\uFFFD")
}

// scoreRun computes the number of correct cases, the percentage score and the
// terminal status, scaled to 0..100.
func scoreRun(cases []model.TestCase, results []model.CaseResult) (int, float64, model.RunStatus) {
	correct := 0

	// Count the number of correct cases by iterating over the results and checking their status.
	for _, r := range results {
		if r.Status == model.CaseCorrect {
			correct++
		}
	}

	// If there are no test cases, we return a score of 100 and a completed status.
	if len(cases) == 0 {
		return 0, 100, model.RunCompleted
	}

	// Calculate the score as a percentage of correct cases out of total cases.
	score := 100 * float64(correct) / float64(len(cases))

	// If all cases are correct, we return the number of correct cases, the score, and a status indicating that the run is completed.
	if correct == len(cases) {
		return correct, score, model.RunCompleted
	}

	// If not all cases are correct, we return the number of correct cases, the score, and a status indicating that the run is uncompleted.
	return correct, score, model.RunUncompleted
}
