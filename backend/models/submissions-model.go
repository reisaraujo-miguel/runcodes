package models

import "time"

// CreateSubmissionResponse is returned when a submission is accepted and
// queued for judging.
type CreateSubmissionResponse struct {
	CommitID int64 `json:"commit_id"`
	// Number is the caller's 1-based submission position for this exercise.
	// Omitted when the number could not be read; the snapshot carries it too.
	Number    int64  `json:"number,omitempty"`
	Status    string `json:"status"`
	EventsURL string `json:"events_url"`
}

// SubmissionSummary is one entry in a user's submission history. It carries
// enough metadata to identify and download a past solution without exposing the
// internal object key.
type SubmissionSummary struct {
	ID int64 `json:"id"`
	// Number is the 1-based position among this user's submissions to this
	// exercise, not the platform-wide commit id.
	Number          int64     `json:"number"`
	Status          string    `json:"status"`
	NumCorrectCases int       `json:"num_correct_cases"`
	Score           float64   `json:"score"`
	CreatedAt       time.Time `json:"created_at"`
	Filename        string    `json:"filename"`
}

// Commit is the persisted state of a submission. It is sent to clients inside
// the SSE "snapshot" event so a reloading client can render the current state
// without waiting for new judge events.
type Commit struct {
	ID int64 `json:"id"`
	// Number is the 1-based position among this user's submissions to this
	// exercise, not the platform-wide commit id.
	Number              int64      `json:"number"`
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
	ExerciseTestCaseID int64    `json:"exercise_test_case_id"`
	CPUTime            float64  `json:"cpu_time"`
	WallTime           *float64 `json:"wall_time"`
	MemUsage           int64    `json:"mem_usage"`
	UserOutput         *string  `json:"user_output"`
	UserOutputType     string   `json:"user_output_type"`
	Status             string   `json:"status"`
	// Reason classifies an interrupted case: timeout, cpu_limit, memory_limit,
	// stack_limit, file_size_limit, signal, monitor_error, output_error or
	// server_error. Nil on a graded case and on an ordinary non-zero exit.
	Reason        *string `json:"reason"`
	StatusMessage *string `json:"status_message"`
	ErrorMessage  *string `json:"error_message"`
}

// SubmissionArtifact identifies a generated file the download endpoint can
// serve for a commit. The run-level entry (`kind` only) says at least one file
// exists; a per-case entry carries the state of that case's upload and, when it
// failed, the reason the download is missing.
type SubmissionArtifact struct {
	Kind       string `json:"kind"`
	TestCaseID *int64 `json:"test_case_id,omitempty"`
	State      string `json:"state,omitempty"`
	Reason     string `json:"reason,omitempty"`
	LimitBytes *int64 `json:"limit_bytes,omitempty"`
}

// Snapshot is the first SSE event sent to a subscriber. Artifact states are
// persisted as the judge reports them, so the files section survives a page
// reload and can still explain a failed upload.
type Snapshot struct {
	Type      string               `json:"type"`
	Commit    *Commit              `json:"commit"`
	Results   []CaseResult         `json:"results"`
	Artifacts []SubmissionArtifact `json:"artifacts"`
}
