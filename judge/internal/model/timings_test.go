package model

import (
	"errors"
	"testing"
	"time"
)

func TestRunTimingsMeasureAndRecord(t *testing.T) {
	timings := NewRunTimings()

	done := timings.Measure(PhaseWorkspacePrepare)
	time.Sleep(2 * time.Millisecond)
	done()

	wantErr := errors.New("boom")
	if err := timings.MeasureFunc(PhaseSourceDownload, func() error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("MeasureFunc error = %v, want %v", err, wantErr)
	}

	timings.RecordCPU(PhaseCompile, 1200*time.Millisecond, 900*time.Millisecond)

	phases := timings.Phases()
	if len(phases) != 3 {
		t.Fatalf("got %d phases, want 3", len(phases))
	}
	if phases[0].Phase != PhaseWorkspacePrepare || phases[0].Duration <= 0 {
		t.Errorf("workspace timing = %+v, want a positive duration", phases[0])
	}
	if phases[0].StartedAt.IsZero() || phases[0].FinishedAt.IsZero() {
		t.Errorf("measured phase has no timestamps: %+v", phases[0])
	}
	if phases[1].Phase != PhaseSourceDownload {
		t.Errorf("second phase = %q, want %q", phases[1].Phase, PhaseSourceDownload)
	}
	if !phases[2].HasCPU || phases[2].CPU != 900*time.Millisecond {
		t.Errorf("compile CPU = %+v, want 900ms", phases[2])
	}
}
