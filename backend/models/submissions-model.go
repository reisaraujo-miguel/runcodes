package models

import "time"

// CreateSubmissionResponse is returned when a submission is accepted and
// queued for judging.
type CreateSubmissionResponse struct {
	CommitID  int64  `json:"commit_id"`
	Status    string `json:"status"`
	EventsURL string `json:"events_url"`
}

// Commit is the persisted state of a submission. It is sent to clients inside
// the SSE "snapshot" event so a reloading client can render the current state
// without waiting for new judge events.
type Commit struct {
	ID                  int64      `json:"id"`
	UserID              *int64     `json:"user_id"`
	ExerciseID          *int64     `json:"exercise_id"`
	Status              string     `json:"status"`
	NumCorrectCases     int        `json:"num_correct_cases"`
	Score               float64    `json:"score"`
	Compiled            *bool      `json:"compiled"`
	CompilationMessage  *string    `json:"compilation_message"`
	CompilationError    *string    `json:"compilation_error"`
	CompilationStarted  *time.Time `json:"compilation_started"`
	CompilationFinished *time.Time `json:"compilation_finished"`
	CreatedAt           time.Time  `json:"created_at"`
	S3Key               *string    `json:"s3_key"`
	IP                  *string    `json:"ip"`
}

// CaseResult is a persisted per-test-case result, as sent in the snapshot.
type CaseResult struct {
	ExerciseTestCaseID int64   `json:"exercise_test_case_id"`
	CPUTime            float64 `json:"cpu_time"`
	MemUsage           int64   `json:"mem_usage"`
	UserOutput         *string `json:"user_output"`
	UserOutputType     string  `json:"user_output_type"`
	Status             string  `json:"status"`
	StatusMessage      *string `json:"status_message"`
	ErrorMessage       *string `json:"error_message"`
}

// Snapshot is the first SSE event sent to a subscriber.
type Snapshot struct {
	Type    string       `json:"type"`
	Commit  *Commit      `json:"commit"`
	Results []CaseResult `json:"results"`
}
