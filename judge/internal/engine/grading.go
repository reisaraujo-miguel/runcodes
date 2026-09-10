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

// readMonitor parses the monitor's INI-ish output (`<id>.monitor_out`). Keys
// live under an `[info]` section; the parser is deliberately lenient.
func readMonitor(path string) monitorInfo {
	var info monitorInfo
	raw, err := os.ReadFile(path)
	if err != nil {
		return info
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
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
	for _, tc := range ws.TestCases {
		result, err := e.gradeCase(ctx, tc, ws)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (e *Engine) gradeCase(ctx context.Context, tc model.TestCase, ws *workspace) (model.CaseResult, error) {
	result := model.CaseResult{
		TestCaseID: tc.ID,
		MemUsage:   -1,
		OutputType: "text",
	}

	outputPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.output", tc.ID))
	errorPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.error", tc.ID))
	monitorPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.monitor_out", tc.ID))

	info := readMonitor(monitorPath)
	result.CPUTime = info.Time
	if info.HasMem {
		result.MemUsage = info.Mem
	}

	errContent := readLimited(errorPath, e.cfg.MaxOutputFileSize)
	if info.Signal != "" || errContent != "" {
		result.Status = model.CaseKilledBySignal
		result.StatusMsg = info.Signal
		result.ErrorMessage = errContent
	} else {
		expectedPath := filepath.Join(ws.BaseDir, fmt.Sprintf("%d.out", tc.ID))
		if err := e.s3.FetchCaseOutput(ctx, tc.ID, expectedPath); err != nil {
			return result, fmt.Errorf("download expected output of case %d: %w", tc.ID, err)
		}
		result.Status = compareOutput(outputPath, expectedPath, tc.ExpectedOutputType)
	}

	result.UserOutput = readLimited(outputPath, e.cfg.MaxOutputFileSize)
	return result, nil
}

// compareOutput maps the legacy comparison modes onto the new schema:
// `file` expected outputs are compared byte-for-byte, `text` ones with the
// strict/lenient text comparators.
func compareOutput(userPath, expectedPath, expectedType string) model.CaseStatus {
	if expectedType == "file" {
		if filesEqual(userPath, expectedPath) {
			return model.CaseCorrect
		}
		return model.CaseKilledBySignal
	}
	switch {
	case cmp.TextEqual(userPath, expectedPath):
		return model.CaseCorrect
	case cmp.TextLenient(userPath, expectedPath):
		return model.CaseBadFormat
	default:
		return model.CaseKilledBySignal
	}
}

func filesEqual(a, b string) bool {
	ra, errA := os.ReadFile(a)
	rb, errB := os.ReadFile(b)
	if errA != nil || errB != nil {
		return false
	}
	return bytes.Equal(ra, rb)
}

// readLimited returns at most max bytes of a file, replacing invalid UTF-8 so
// it stays safe to store in a text column. A missing file yields "".
func readLimited(path string, max int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var r io.Reader = f
	if max > 0 {
		r = io.LimitReader(f, max)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return ""
	}
	return strings.ToValidUTF8(string(raw), "\uFFFD")
}

// scoreRun computes the number of correct cases, the percentage score and the
// terminal status, following the legacy engine's semantics scaled to 0..100.
func scoreRun(cases []model.TestCase, results []model.CaseResult) (int, float64, model.RunStatus) {
	correct := 0
	for _, r := range results {
		if r.Status == model.CaseCorrect {
			correct++
		}
	}
	if len(cases) == 0 {
		return 0, 100, model.RunCompleted
	}
	score := 100 * float64(correct) / float64(len(cases))
	if correct == len(cases) {
		return correct, score, model.RunCompleted
	}
	return correct, score, model.RunUncompleted
}
