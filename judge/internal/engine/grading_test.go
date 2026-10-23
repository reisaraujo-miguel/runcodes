package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

func TestReadMonitor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1.monitor_out")
	content := "[info]\nexit_status=0\nsignal=\ntime=0.042\ncpu_time=0.031\nmem=123456\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readMonitor(path)
	if info.Signal != "" {
		t.Errorf("signal = %q, want empty", info.Signal)
	}
	if !info.HasExit || info.ExitStatus != 0 {
		t.Errorf("exit_status = %d (has=%v), want 0", info.ExitStatus, info.HasExit)
	}
	if info.Time != 0.042 {
		t.Errorf("time = %v, want 0.042", info.Time)
	}
	if !info.HasCPU || info.CPU != 0.031 {
		t.Errorf("cpu_time = %v (has=%v), want 0.031", info.CPU, info.HasCPU)
	}
	if !info.HasMem || info.Mem != 123456 {
		t.Errorf("mem = %d (has=%v), want 123456", info.Mem, info.HasMem)
	}
}

func TestReadMonitorMissing(t *testing.T) {
	info := readMonitor(filepath.Join(t.TempDir(), "nope"))
	if info.Signal != "" || info.Time != 0 || info.HasCPU || info.HasMem || info.HasExit {
		t.Fatalf("expected zero value, got %+v", info)
	}
}

func TestExitFailureMessage(t *testing.T) {
	tests := []struct {
		name       string
		info       monitorInfo
		wantMsg    string
		wantFailed bool
	}{
		{"success", monitorInfo{ExitStatus: 0, HasExit: true}, "", false},
		{"no report", monitorInfo{}, "", false},
		{"timeout", monitorInfo{ExitStatus: 124, HasExit: true}, "timeout", true},
		{"sigkill", monitorInfo{ExitStatus: 137, HasExit: true}, "killed by signal 9", true},
		{"non-zero", monitorInfo{ExitStatus: 3, HasExit: true}, "exit status 3", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, failed := exitFailureMessage(tc.info)
			if msg != tc.wantMsg || failed != tc.wantFailed {
				t.Fatalf("exitFailureMessage(%+v) = (%q, %v), want (%q, %v)",
					tc.info, msg, failed, tc.wantMsg, tc.wantFailed)
			}
		})
	}
}

// TestGradeCaseTreatsNonZeroExitAsFailure reproduces the silent wrong-grade
// path: the program wrote the expected output, but the monitor says it timed out
// (or exited non-zero). The verdict must be a failure, not `correct`.
func TestGradeCaseTreatsNonZeroExitAsFailure(t *testing.T) {
	tests := []struct {
		name       string
		exitStatus string
		wantMsg    string
	}{
		{"timeout", "124", "timeout"},
		{"non-zero exit", "1", "exit status 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			ws := &workspace{BaseDir: dir}

			if err := os.WriteFile(filepath.Join(dir, "7.output"), []byte("42\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			monitor := "[info]\nexit_status=" + tt.exitStatus + "\nsignal=\ntime=0.01\n"
			if err := os.WriteFile(filepath.Join(dir, "7.monitor_out"), []byte(monitor), 0o644); err != nil {
				t.Fatal(err)
			}

			e := &Engine{
				cfg: &config.Config{
					MaxOutputFileSize:   1024,
					MaxCompareFileBytes: 1024,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			result, err := e.gradeCase(context.Background(), model.TestCase{ID: 7}, ws)
			if err != nil {
				t.Fatalf("gradeCase: %v", err)
			}
			if result.Status != model.CaseKilledBySignal || result.StatusMsg != tt.wantMsg {
				t.Fatalf("gradeCase = status %q msg %q, want %q %q",
					result.Status, result.StatusMsg, model.CaseKilledBySignal, tt.wantMsg)
			}
		})
	}
}

func TestScoreRun(t *testing.T) {
	cases := []model.TestCase{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}

	correct, score, status := scoreRun(cases, []model.CaseResult{
		{Status: model.CaseCorrect},
		{Status: model.CaseCorrect},
		{Status: model.CaseKilledBySignal},
		{Status: model.CaseBadFormat},
	})
	if correct != 2 || score != 50 || status != model.RunUncompleted {
		t.Fatalf("got correct=%d score=%v status=%s", correct, score, status)
	}

	correct, score, status = scoreRun(cases, []model.CaseResult{
		{Status: model.CaseCorrect}, {Status: model.CaseCorrect},
		{Status: model.CaseCorrect}, {Status: model.CaseCorrect},
	})
	if correct != 4 || score != 100 || status != model.RunCompleted {
		t.Fatalf("got correct=%d score=%v status=%s", correct, score, status)
	}

	// No test cases: nothing was graded, so this is not a full score.
	_, score, status = scoreRun(nil, nil)
	if score != 0 || status != model.RunUncompleted {
		t.Fatalf("empty run got score=%v status=%s", score, status)
	}
}

// TestGradeCaseIgnoresStderrWhenAnswerIsCorrect covers two contracts: a
// diagnostic on stderr does not fail a case, and a text expected output is read
// from the database rather than requiring an S3 object. The nil S3 client is
// deliberate: touching S3 here would panic.
func TestGradeCaseIgnoresStderrWhenAnswerIsCorrect(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace{BaseDir: dir}
	if err := os.WriteFile(filepath.Join(dir, "7.output"), []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "7.error"), []byte("warning: harmless\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "7.monitor_out"), []byte("[info]\nexit_status=0\nsignal=\ntime=0.01\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &Engine{
		cfg: &config.Config{
			MaxOutputFileSize:   1024,
			MaxCompareFileBytes: 1024,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	result, err := e.gradeCase(context.Background(), model.TestCase{
		ID:                 7,
		ExpectedOutputType: "text",
		ExpectedOutput:     "42\n",
	}, ws)
	if err != nil {
		t.Fatalf("gradeCase: %v", err)
	}
	if result.Status != model.CaseCorrect {
		t.Fatalf("gradeCase status = %s, want correct", result.Status)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("ErrorMessage = %q, want empty for a successful case", result.ErrorMessage)
	}
}
