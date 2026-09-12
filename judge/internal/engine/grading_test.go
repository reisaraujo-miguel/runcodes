package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runcodes-icmc/judge/internal/model"
)

func TestReadMonitor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1.monitor_out")
	content := "[info]\nsignal=\ntime=0.042\nmem=123456\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	info := readMonitor(path)
	if info.Signal != "" {
		t.Errorf("signal = %q, want empty", info.Signal)
	}
	if info.Time != 0.042 {
		t.Errorf("time = %v, want 0.042", info.Time)
	}
	if !info.HasMem || info.Mem != 123456 {
		t.Errorf("mem = %d (has=%v), want 123456", info.Mem, info.HasMem)
	}
}

func TestReadMonitorMissing(t *testing.T) {
	info := readMonitor(filepath.Join(t.TempDir(), "nope"))
	if info.Signal != "" || info.Time != 0 || info.HasMem {
		t.Fatalf("expected zero value, got %+v", info)
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

	// No test cases: the legacy engine treated this as a full score.
	_, score, status = scoreRun(nil, nil)
	if score != 100 || status != model.RunCompleted {
		t.Fatalf("empty run got score=%v status=%s", score, status)
	}
}
