package models

import "time"

/*
Limits are the four limits applied to a test case under one language.

A zero is meaningful. For the three byte-valued limits it means "no limit", and
it is different from the platform default; the CPU time limit is always positive,
because a case needs a wall-clock budget.
*/
type Limits struct {
	CPUTimeLimitSeconds int   `json:"cpu_time_limit_seconds"`
	MemUsageLimitBytes  int64 `json:"mem_usage_limit_bytes"`
	StackLimitBytes     int64 `json:"stack_limit_bytes"`
	FileSizeLimitBytes  int64 `json:"file_size_limit_bytes"`
}

// LimitSource names the layer a limit came from.
type LimitSource string

const (
	LimitSourcePlatform LimitSource = "platform"
	LimitSourceExercise LimitSource = "exercise"
	LimitSourceCase     LimitSource = "case"
)

// LimitSources records, for each limit, which layer supplied it.
type LimitSources struct {
	CPUTimeLimitSeconds LimitSource `json:"cpu_time_limit_seconds"`
	MemUsageLimitBytes  LimitSource `json:"mem_usage_limit_bytes"`
	StackLimitBytes     LimitSource `json:"stack_limit_bytes"`
	FileSizeLimitBytes  LimitSource `json:"file_size_limit_bytes"`
}

// LimitOverrides is one layer's values. A nil field inherits the layer below;
// the platform default is the last layer and never inherits.
type LimitOverrides struct {
	CPUTimeLimitSeconds *int   `json:"cpu_time_limit_seconds"`
	MemUsageLimitBytes  *int64 `json:"mem_usage_limit_bytes"`
	StackLimitBytes     *int64 `json:"stack_limit_bytes"`
	FileSizeLimitBytes  *int64 `json:"file_size_limit_bytes"`
}

// Empty reports whether the layer overrides nothing.
func (o LimitOverrides) Empty() bool {
	return o.CPUTimeLimitSeconds == nil && o.MemUsageLimitBytes == nil &&
		o.StackLimitBytes == nil && o.FileSizeLimitBytes == nil
}

// LanguageLimits is one allowed language of an exercise, with the overrides
// declared at the layer being read and the limits that end up applying.
type LanguageLimits struct {
	AllowedFileTypeID int64          `json:"allowed_file_type_id"`
	Name              string         `json:"name"`
	Extension         string         `json:"extension"`
	Overrides         LimitOverrides `json:"overrides"`
	Limits            Limits         `json:"limits"`
	Sources           LimitSources   `json:"sources"`
}

// PlatformLimits is the default set every language inherits.
type PlatformLimits struct {
	Limits
	// CompilationTimeoutSeconds is the time a compilation may take; it is not a
	// test case limit, but it is part of the same policy.
	CompilationTimeoutSeconds int `json:"compilation_timeout_seconds"`
}

// ExerciseLimitAlert is one warning shown to the exercise owner: submissions
// have been failing because of a limit on one language.
type ExerciseLimitAlert struct {
	ID                int64   `json:"id"`
	ExerciseID        int64   `json:"exercise_id"`
	AllowedFileTypeID *int64  `json:"allowed_file_type_id"`
	LanguageName      *string `json:"language_name"`
	// LimitName is cpu_time, memory, stack, file_size or timeout.
	LimitName       string     `json:"limit_name"`
	Failures        int        `json:"failures"`
	WindowStartedAt time.Time  `json:"window_started_at"`
	WindowEndedAt   time.Time  `json:"window_ended_at"`
	CreatedAt       time.Time  `json:"created_at"`
	AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
}
