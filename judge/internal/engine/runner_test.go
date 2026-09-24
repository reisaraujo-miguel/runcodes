package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/runcodes-icmc/judge/internal/model"
)

// TestClassifyMapsOnlyPhaseTimeoutsToTimeout pins the distinction between a
// submission that ran out of time and a run the judge could not finish. The
// per-phase `timeoutError` is the submission's own TLE; a cancelled or expired
// run context comes from the judge's lifecycle (shutdown, or the per-run budget),
// and reporting that as a timeout would blame the student for an infrastructure
// event.
func TestClassifyMapsOnlyPhaseTimeoutsToTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want model.RunStatus
	}{
		{"phase timeout", &timeoutError{msg: "timed out waiting for \"run.done\""}, model.RunTimeout},
		{"wrapped phase timeout", fmt.Errorf("await: %w", &timeoutError{msg: "x"}), model.RunTimeout},
		{"deadline exceeded", context.DeadlineExceeded, model.RunServerError},
		{"cancelled", context.Canceled, model.RunServerError},
		{"wrapped cancellation", fmt.Errorf("run container: %w", context.Canceled), model.RunServerError},
		{"any other failure", errors.New("podman unavailable"), model.RunServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.err); got != tc.want {
				t.Fatalf("classify(%v) = %s, want %s", tc.err, got, tc.want)
			}
		})
	}
}
