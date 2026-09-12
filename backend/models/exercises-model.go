package models

import "time"

// AllowedFileType is one entry of the platform-wide allowed file type catalog.
type AllowedFileType struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Extension    string `json:"extension"`
	IsCompilable bool   `json:"is_compilable"`
	IsAvailable  bool   `json:"is_available"`
}

// CreateExerciseRequest is the JSON body of POST
// /api/v1/offerings/{offeringId}/exercises.
type CreateExerciseRequest struct {
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Deadline           *time.Time `json:"deadline"`
	OpenDate           *time.Time `json:"open_date"`
	ShowBeforeOpenDate *bool      `json:"show_before_open_date"`
	AllowedFileTypeIDs []int64    `json:"allowed_file_type_ids"`
}

// UpdateExerciseRequest is the JSON body of PUT /api/v1/exercises/{id}. Every
// field is optional: absent fields are left untouched (partial update).
type UpdateExerciseRequest struct {
	Title              *string    `json:"title"`
	Description        *string    `json:"description"`
	Deadline           *time.Time `json:"deadline"`
	OpenDate           *time.Time `json:"open_date"`
	ShowBeforeOpenDate *bool      `json:"show_before_open_date"`
	AllowedFileTypeIDs *[]int64   `json:"allowed_file_type_ids"`
}

// Exercise is an exercise as returned by the API.
type Exercise struct {
	ID                 int64             `json:"id"`
	OfferingID         int64             `json:"offering_id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Deadline           time.Time         `json:"deadline"`
	OpenDate           time.Time         `json:"open_date"`
	ShowBeforeOpenDate bool              `json:"show_before_open_date"`
	Removed            bool              `json:"removed"`
	AllowedFileTypeIDs []int64           `json:"allowed_file_type_ids"`
	AllowedFileTypes   []AllowedFileType `json:"allowed_file_types"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// TestCaseFile is an extra file attached to a test case. Path is the basename
// the file is stored and materialised under.
type TestCaseFile struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// TestCase is a single test case as returned by the API. When requested by a
// student, fields whose show_* flag is false are blanked out.
type TestCase struct {
	ID                  int64          `json:"id"`
	ExerciseID          int64          `json:"exercise_id"`
	Input               string         `json:"input"`
	InputType           string         `json:"input_type"`
	ShowInput           bool           `json:"show_input"`
	ExpectedOutput      string         `json:"expected_output"`
	ExpectedOutputType  string         `json:"expected_output_type"`
	ShowExpectedOutput  bool           `json:"show_expected_output"`
	ShowUserOutput      bool           `json:"show_user_output"`
	CPUTimeLimitSeconds int            `json:"cpu_time_limit_seconds"`
	MemUsageLimitBytes  int64          `json:"mem_usage_limit_bytes"`
	StackLimitBytes     int64          `json:"stack_limit_bytes"`
	FileSizeLimitBytes  int64          `json:"file_size_limit_bytes"`
	Files               []TestCaseFile `json:"files"`
}

// CompilationFile is an extra source file compiled together with a submission.
// Filename mirrors Path and exists for the frontend's convenience.
type CompilationFile struct {
	ID         int64  `json:"id"`
	ExerciseID int64  `json:"exercise_id"`
	Path       string `json:"path"`
	Filename   string `json:"filename"`
}

// AttachedFile is a material attached to an exercise for students to download.
type AttachedFile struct {
	ID         int64  `json:"id"`
	ExerciseID int64  `json:"exercise_id"`
	Path       string `json:"path"`
	Filename   string `json:"filename"`
}
