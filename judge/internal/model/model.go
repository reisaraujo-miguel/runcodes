// Package model contains the judge's domain types.
package model

import "time"

// Commit is a queued submission claimed from Postgres.
type Commit struct {
	ID             int64
	UserID         int64
	ExerciseID     int64
	RealExerciseID int64
	S3Key          string
	// QueueWait is how long the row sat in the queue before this claim,
	// measured entirely on the database clock: the claim's now() minus the row's
	// created_at. The judge's own clock is deliberately not part of it; the two
	// hosts can disagree, and the queue phase would silently absorb the skew.
	QueueWait time.Duration
	// HasQueueWait reports whether the claim reported QueueWait. It is false only
	// for a commit built without one (tests).
	HasQueueWait bool
}

// Filename is the base name of the submitted source object.
func (c Commit) Filename() string {
	return baseName(c.S3Key)
}

// baseName returns the last path component of a key, or the key itself if it has no slashes.
// This is like path.Base but without the OS-specific behavior.
func baseName(key string) string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[i+1:]
		}
	}

	return key
}

// TestCase mirrors a row of `exercises_test_cases`.
type TestCase struct {
	ID         int64
	ExerciseID int64
	// Number is the case's 1-based position inside its exercise. IDs are
	// platform-wide, so the number is what the student sees and what generated
	// files are named after. Zero means the case was built without one.
	Number             int
	InputType          string // "text" | "file"
	Input              string // inline input for InputType == "text"
	ExpectedOutputType string // "text" | "file"
	ExpectedOutput     string // inline expected output for ExpectedOutputType == "text"
	ShowInput          bool
	ShowExpectedOutput bool
	ShowUserOutput     bool
	CPUTimeLimit       int
	MemUsageLimit      int64
	StackLimit         int64
	FileSizeLimit      int64
	Files              []string
}

// CaseStatus is the verdict for one test case (maps to the DB enum).
type CaseStatus string

// Case verdicts, matching `commit_exercise_test_case_status_t`.
const (
	CaseCorrect        CaseStatus = "correct"
	CaseBadFormat      CaseStatus = "bad_formatted_output"
	CaseKilledBySignal CaseStatus = "killed_with_signal"
)

// CaseReason explains why an interrupted case did not produce a verdict.
// It mirrors `case_interruption_reason_t`; an empty reason means the case was
// graded normally (correct or badly formatted) or failed without an interruption
// the sandbox can attribute (a non-zero exit status).
type CaseReason string

const (
	CaseReasonTimeout       CaseReason = "timeout"
	CaseReasonCPULimit      CaseReason = "cpu_limit"
	CaseReasonMemoryLimit   CaseReason = "memory_limit"
	CaseReasonStackLimit    CaseReason = "stack_limit"
	CaseReasonFileSizeLimit CaseReason = "file_size_limit"
	CaseReasonSignal        CaseReason = "signal"
	CaseReasonMonitor       CaseReason = "monitor_error"
	CaseReasonOutput        CaseReason = "output_error"
	CaseReasonServer        CaseReason = "server_error"
)

// CaseResult is the graded outcome of one test case.
type CaseResult struct {
	TestCaseID   int64
	CPUTime      float64
	WallTime     float64
	MemUsage     int64
	Status       CaseStatus
	Reason       CaseReason
	StatusMsg    string
	UserOutput   string
	OutputType   string // "text" | "file"
	ErrorMessage string
}

// QueueStats is a snapshot of the durable queue, used by the judge metrics
// callback. Depth counts rows in the `queued` state; OldestSeconds is the age of
// the oldest one (0 when the queue is empty).
type QueueStats struct {
	Depth         int64
	OldestSeconds float64
}

// RunStatus is the terminal status of a submission (maps to the DB enum).
type RunStatus string

// Terminal run statuses, matching `commit_status_t`.
const (
	RunCompleted        RunStatus = "completed"
	RunUncompleted      RunStatus = "uncompleted"
	RunCompilationError RunStatus = "compilation_error"
	RunServerError      RunStatus = "server_error"
	RunTimeout          RunStatus = "timeout"
)

// Alert severities and codes carried by `alert` events. The backend stores them
// in `system_alerts` and emails every administrator; keep the values in step
// with backend/alerts/alerts.go and the table's CHECK constraints.
const (
	AlertSeverityWarning  = "warning"
	AlertSeverityCritical = "critical"

	// AlertStartupTimeout reports a container that did not print its first
	// milestone within JUDGE_STARTUP_WAIT. Every retry of the same submission
	// fails the same way until an administrator acts on it.
	AlertStartupTimeout = "startup_timeout"
)
