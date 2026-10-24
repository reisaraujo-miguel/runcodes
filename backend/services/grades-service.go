package services

import (
	"context"
	"database/sql"
	"log/slog"
	"math"
	"time"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
GetOfferingGradebook builds the grade table of an offering: one row per enrolled
student, one column per exercise, plus the offering and per-exercise statistics.

The grade of a student on an exercise is the best score among their submissions.
The offering grade is the mean of those best scores over every exercise of the
offering, counting an exercise without a submission as zero. That is the number
the professor and the student see, so the statistics and the CSV cannot drift
from the table.
*/
func GetOfferingGradebook(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.OfferingGradebook, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		// A student must not learn the grade table of their classmates. The
		// offering is reported as missing, like every other author-only view.
		return nil, ErrOfferingNotFound
	}

	return buildOfferingGradebook(ctx, rel.Offering.ID, rel.Offering.Name)
}

// gradeCell is the internal aggregation of a student's submissions on one
// exercise.
type gradeCell struct {
	score            float64
	submissionCount  int64
	lastSubmissionAt time.Time
}

func buildOfferingGradebook(
	ctx context.Context, offeringID int64, offeringName string,
) (*models.OfferingGradebook, error) {
	exercises, err := loadGradebookExercises(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	students, err := loadGradebookStudents(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	cellByStudent, err := loadGradeCells(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	gradebook := &models.OfferingGradebook{
		OfferingID:         offeringID,
		OfferingName:       offeringName,
		Exercises:          exercises,
		Students:           make([]models.GradebookStudent, 0, len(students)),
		Summary:            models.GradeSummary{StudentCount: len(students)},
		ExerciseStatistics: make([]models.ExerciseStatistics, 0, len(exercises)),
	}

	// Per-exercise accumulators, filled while the student rows are built.
	type exerciseAccumulator struct {
		submitted    int
		above50      int
		scoreSum     float64
		submittedSum float64
		best         *float64
		lowest       *float64
	}
	accumulators := make([]exerciseAccumulator, len(exercises))

	for _, student := range students {
		cells := make([]models.GradebookCell, 0, len(exercises))
		var (
			scoreSum        float64
			submissionCount int64
			lastSubmission  *time.Time
		)

		for index, exercise := range exercises {
			cell := models.GradebookCell{ExerciseID: exercise.ID}
			aggregated, ok := cellByStudent[student.ID][exercise.ID]
			if ok {
				score := roundGradeScore(aggregated.score)
				cell.Score = &score
				cell.SubmissionCount = aggregated.submissionCount
				submittedAt := aggregated.lastSubmissionAt
				cell.LastSubmissionAt = &submittedAt

				scoreSum += score
				submissionCount += aggregated.submissionCount
				if lastSubmission == nil || submittedAt.After(*lastSubmission) {
					lastSubmission = &submittedAt
				}

				accumulator := &accumulators[index]
				accumulator.submitted++
				accumulator.scoreSum += score
				accumulator.submittedSum += score
				if score > 50 {
					accumulator.above50++
				}
				if accumulator.best == nil || score > *accumulator.best {
					accumulator.best = &score
				}
				if accumulator.lowest == nil || score < *accumulator.lowest {
					accumulator.lowest = &score
				}
			}
			cells = append(cells, cell)
		}

		average := 0.0
		if len(exercises) > 0 {
			average = roundGradeScore(scoreSum / float64(len(exercises)))
		}

		gradebook.Students = append(gradebook.Students, models.GradebookStudent{
			UserID:           student.ID,
			Name:             student.Name,
			Email:            student.Email,
			Scores:           cells,
			Average:          average,
			SubmissionCount:  submissionCount,
			LastSubmissionAt: lastSubmission,
		})

		gradebook.Summary.GlobalAverage += average
		gradebook.Summary.TotalSubmissions += submissionCount
		if submissionCount > 0 {
			gradebook.Summary.StudentsWithSubmission++
		}
		if average > 50 {
			gradebook.Summary.StudentsAbove50++
		}
	}

	if len(students) > 0 {
		gradebook.Summary.GlobalAverage = roundGradeScore(
			gradebook.Summary.GlobalAverage / float64(len(students)),
		)
	}

	for index, exercise := range exercises {
		accumulator := accumulators[index]
		statistics := models.ExerciseStatistics{
			ExerciseID:        exercise.ID,
			Title:             exercise.Title,
			Deadline:          exercise.Deadline,
			StudentCount:      len(students),
			SubmittedStudents: accumulator.submitted,
			StudentsAbove50:   accumulator.above50,
			BestScore:         accumulator.best,
			LowestScore:       accumulator.lowest,
		}

		if len(students) > 0 {
			statistics.Average = roundGradeScore(
				accumulator.scoreSum / float64(len(students)),
			)
			statistics.ApprovalRate = roundGradeScore(
				100 * float64(accumulator.above50) / float64(len(students)),
			)
		}
		if accumulator.submitted > 0 {
			average := roundGradeScore(
				accumulator.submittedSum / float64(accumulator.submitted),
			)
			statistics.AverageSubmitted = &average
		}

		gradebook.ExerciseStatistics = append(
			gradebook.ExerciseStatistics, statistics,
		)
	}

	return gradebook, nil
}

func loadGradebookExercises(
	ctx context.Context, offeringID int64,
) ([]models.GradebookExercise, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, title, deadline
		FROM exercises
		WHERE offering_id = $1 AND NOT removed
		ORDER BY open_date, id`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading gradebook exercises",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	exercises := make([]models.GradebookExercise, 0, 8)
	for rows.Next() {
		var exercise models.GradebookExercise
		if err := rows.Scan(
			&exercise.ID, &exercise.Title, &exercise.Deadline,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning gradebook exercise",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return exercises, nil
}

type gradebookStudent struct {
	ID    int64
	Name  string
	Email string
}

func loadGradebookStudents(
	ctx context.Context, offeringID int64,
) ([]gradebookStudent, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT u.id, u.name, u.email
		FROM enrollments en
		JOIN users u ON u.id = en.user_id
		WHERE en.offering_id = $1
		  AND en.role = 'student'
		  AND NOT en.banned
		ORDER BY u.name, u.id`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading gradebook students",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	students := make([]gradebookStudent, 0, 32)
	for rows.Next() {
		var student gradebookStudent
		if err := rows.Scan(
			&student.ID, &student.Name, &student.Email,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning gradebook student",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		students = append(students, student)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return students, nil
}

// loadGradeCells reads the best score, number of submissions and last
// submission of every (student, exercise) pair that has at least one commit.
func loadGradeCells(
	ctx context.Context, offeringID int64,
) (map[int64]map[int64]gradeCell, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT c.user_id, c.exercise_id, MAX(c.score)::float8,
		       COUNT(*), MAX(c.created_at)
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		WHERE e.offering_id = $1 AND NOT e.removed
		  AND c.user_id IS NOT NULL
		GROUP BY c.user_id, c.exercise_id`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading grade cells",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	cells := make(map[int64]map[int64]gradeCell)
	for rows.Next() {
		var (
			userID     int64
			exerciseID int64
			score      float64
			count      int64
			last       sql.NullTime
		)
		if err := rows.Scan(
			&userID, &exerciseID, &score, &count, &last,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning grade cell",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		byExercise := cells[userID]
		if byExercise == nil {
			byExercise = make(map[int64]gradeCell)
			cells[userID] = byExercise
		}
		cell := gradeCell{score: score, submissionCount: count}
		if last.Valid {
			cell.lastSubmissionAt = last.Time
		}
		byExercise[exerciseID] = cell
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	return cells, nil
}

// roundGradeScore keeps grades at the two decimals the database stores and the
// UI displays, so sums do not accumulate floating point noise.
func roundGradeScore(value float64) float64 {
	return math.Round(value*100) / 100
}
