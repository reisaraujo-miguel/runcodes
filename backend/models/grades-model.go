package models

import "time"

// GradebookExercise is a column of the grade table: one exercise of the
// offering.
type GradebookExercise struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	Deadline time.Time `json:"deadline"`
}

// GradebookCell is one student's result on one exercise. Score is absent (null)
// when the student never submitted; the offering grade counts that as zero.
type GradebookCell struct {
	ExerciseID       int64      `json:"exercise_id"`
	Score            *float64   `json:"score"`
	SubmissionCount  int64      `json:"submission_count"`
	LastSubmissionAt *time.Time `json:"last_submission_at"`
}

// GradebookStudent is one row of the grade table: a non-banned student of the
// offering with their best score on each exercise and the mean of those scores.
type GradebookStudent struct {
	UserID           int64           `json:"user_id"`
	Name             string          `json:"name"`
	Email            string          `json:"email"`
	Scores           []GradebookCell `json:"scores"`
	Average          float64         `json:"average"`
	SubmissionCount  int64           `json:"submission_count"`
	LastSubmissionAt *time.Time      `json:"last_submission_at"`
}

// GradeSummary aggregates the grade table for the offering as a whole.
type GradeSummary struct {
	StudentCount           int     `json:"student_count"`
	StudentsWithSubmission int     `json:"students_with_submission"`
	StudentsAbove50        int     `json:"students_above_50"`
	GlobalAverage          float64 `json:"global_average"`
	TotalSubmissions       int64   `json:"total_submissions"`
}

// ExerciseStatistics aggregates one exercise of the offering. The average counts
// every student (a missing submission is zero), while AverageSubmitted is the
// mean among the students who actually submitted and is omitted when nobody did.
type ExerciseStatistics struct {
	ExerciseID        int64     `json:"exercise_id"`
	Title             string    `json:"title"`
	Deadline          time.Time `json:"deadline"`
	StudentCount      int       `json:"student_count"`
	SubmittedStudents int       `json:"submitted_students"`
	StudentsAbove50   int       `json:"students_above_50"`
	Average           float64   `json:"average"`
	AverageSubmitted  *float64  `json:"average_submitted"`
	ApprovalRate      float64   `json:"approval_rate"`
	BestScore         *float64  `json:"best_score"`
	LowestScore       *float64  `json:"lowest_score"`
}

// OfferingGradebook is the complete grade table of one offering, with the
// statistics derived from it. It backs both the professor's grade tab and the
// CSV export.
type OfferingGradebook struct {
	OfferingID         int64                `json:"offering_id"`
	OfferingName       string               `json:"offering_name"`
	Exercises          []GradebookExercise  `json:"exercises"`
	Students           []GradebookStudent   `json:"students"`
	Summary            GradeSummary         `json:"summary"`
	ExerciseStatistics []ExerciseStatistics `json:"exercise_statistics"`
}
