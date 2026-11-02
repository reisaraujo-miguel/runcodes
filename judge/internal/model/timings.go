package model

import "time"

// Phase names recorded in `commit_phase_timings`. The backend stores the same
// strings in its enum; keep the two lists in step.
const (
	PhaseQueue            = "queue"
	PhaseWorkspacePrepare = "workspace_prepare"
	PhaseSourceDownload   = "source_download"
	PhaseSourceResolve    = "source_resolve"
	PhaseCompilationFiles = "compilation_files"
	PhaseTestCases        = "test_cases"
	PhaseTestCaseFiles    = "test_case_files"
	PhaseContainerConfig  = "container_config"
	PhaseImageEnsure      = "image_ensure"
	PhaseContainerCreate  = "container_create"
	PhaseContainerStart   = "container_start"
	PhaseStartup          = "startup"
	PhaseCompile          = "compile"
	PhaseRun              = "run"
	PhaseGrade            = "grade"
	PhaseArtifact         = "artifact"
)

// PhaseTiming is one measured phase of a run. Duration is wall time; CPU and
// MemPeak are set only when the source could measure them. StartedAt and
// FinishedAt are zero for durations that came from the monitor's self-contained
// report rather than from judge-side clocks.
type PhaseTiming struct {
	Phase      string
	Duration   time.Duration
	CPU        time.Duration
	HasCPU     bool
	MemPeak    int64
	HasMemPeak bool
	StartedAt  time.Time
	FinishedAt time.Time
	Detail     map[string]any
	Source     string
}

// RunTimings accumulates the phases of one commit in the order they finish.
// It is used by a single run goroutine and is not safe for concurrent use.
type RunTimings struct {
	phases []PhaseTiming
}

// NewRunTimings returns an empty timing set.
func NewRunTimings() *RunTimings {
	return &RunTimings{phases: make([]PhaseTiming, 0, 12)}
}

// Record adds a phase whose duration is already known. It is used for the
// queue wait, where the start and end are not judge-side measurements.
func (t *RunTimings) Record(phase string, duration time.Duration) {
	if t == nil {
		return
	}
	t.append(PhaseTiming{
		Phase:    phase,
		Duration: duration,
		Source:   "judge",
	})
}

// RecordCPU adds a phase whose wall and CPU durations are known (the compile
// phase's monitor report is the main case).
func (t *RunTimings) RecordCPU(phase string, duration, cpu time.Duration) {
	if t == nil {
		return
	}
	t.append(PhaseTiming{
		Phase:    phase,
		Duration: duration,
		CPU:      cpu,
		HasCPU:   true,
		Source:   "monitor",
	})
}

// RecordDetail adds a phase with small metadata (for example the image name and
// whether it had to be pulled).
func (t *RunTimings) RecordDetail(
	phase string, duration time.Duration, detail map[string]any,
) {
	if t == nil {
		return
	}
	t.append(PhaseTiming{
		Phase:    phase,
		Duration: duration,
		Detail:   detail,
		Source:   "judge",
	})
}

// Measure records the time between the returned function and its call. Use it
// when the measured block has to return a value:
//
//	done := timings.Measure(model.PhaseSourceDownload)
//	err := e.s3.FetchCommitSource(...)
//	done()
func (t *RunTimings) Measure(phase string) func() {
	if t == nil {
		return func() {}
	}
	startedAt := time.Now()
	return func() {
		t.append(PhaseTiming{
			Phase:      phase,
			Duration:   time.Since(startedAt),
			StartedAt:  startedAt,
			FinishedAt: time.Now(),
			Source:     "judge",
		})
	}
}

// MeasureFunc records a block that returns an error, preserving the error.
func (t *RunTimings) MeasureFunc(phase string, fn func() error) error {
	if t == nil {
		return fn()
	}
	done := t.Measure(phase)
	err := fn()
	done()
	return err
}

// Phases returns a copy of the accumulated phases, so callers cannot mutate the
// recorder while the event is being published.
func (t *RunTimings) Phases() []PhaseTiming {
	if t == nil {
		return nil
	}
	out := make([]PhaseTiming, len(t.phases))
	copy(out, t.phases)
	return out
}

func (t *RunTimings) append(timing PhaseTiming) {
	if timing.Phase == "" {
		return
	}
	if timing.Duration < 0 {
		timing.Duration = 0
	}
	t.phases = append(t.phases, timing)
}
