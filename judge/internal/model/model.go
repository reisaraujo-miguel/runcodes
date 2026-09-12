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
	CreatedAt      time.Time
}

// Filename is the base name of the submitted source object.
func (c Commit) Filename() string {
	return baseName(c.S3Key)
}

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
	ID                 int64
	ExerciseID         int64
	InputType          string // "text" | "file"
	ExpectedOutputType string // "text" | "file"
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

// CaseResult is the graded outcome of one test case.
type CaseResult struct {
	TestCaseID   int64
	CPUTime      float64
	MemUsage     int64
	Status       CaseStatus
	StatusMsg    string
	UserOutput   string
	OutputType   string // "text" | "file"
	ErrorMessage string
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
