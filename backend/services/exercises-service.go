package services

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
)

func CreateExercise(
	ctx context.Context, offeringID int64, req *models.CreateExerciseRequest,
	claims map[string]any,
) (*models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		return nil, ErrNotOwner
	}

	if err := validateAllowedFileTypeIDs(ctx, req.AllowedFileTypeIDs); err != nil {
		return nil, err
	}

	var deadline, openDate time.Time
	if req.Deadline != nil {
		deadline = *req.Deadline
	}
	if req.OpenDate != nil {
		openDate = *req.OpenDate
	}
	if err := ValidateExerciseFields(req.Title, deadline, openDate); err != nil {
		return nil, err
	}

	showBefore := false
	if req.ShowBeforeOpenDate != nil {
		showBefore = *req.ShowBeforeOpenDate
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing exercise transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	exercise := models.Exercise{
		OfferingID:         offeringID,
		Title:              strings.TrimSpace(req.Title),
		Description:        req.Description,
		Deadline:           deadline,
		OpenDate:           openDate,
		ShowBeforeOpenDate: showBefore,
		AllowedFileTypeIDs: []int64{},
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO exercises
		    (offering_id, creator_id, title, description, deadline, open_date,
		     show_before_open_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		offeringID, userID, exercise.Title, exercise.Description,
		exercise.Deadline, exercise.OpenDate, exercise.ShowBeforeOpenDate,
	).Scan(&exercise.ID, &exercise.CreatedAt, &exercise.UpdatedAt)
	if err != nil {
		slog.ErrorContext(ctx, "error inserting exercise",
			slog.Int64("offering_id", offeringID),
			slog.Any("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if err := insertAllowedFileTypes(ctx, tx, exercise.ID, req.AllowedFileTypeIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing exercise transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	exercise.AllowedFileTypeIDs = dedupeIDs(req.AllowedFileTypeIDs)
	if exs := []models.Exercise{exercise}; attachAllowedFileTypes(ctx, exs) == nil {
		exercise = exs[0]
	}
	cache.Delete(ctx, cache.OfferingExercisesKey(offeringID))

	slog.InfoContext(ctx, "exercise created",
		slog.Int64("exercise_id", exercise.ID),
		slog.Int64("offering_id", offeringID),
		slog.Any("user_id", userID),
	)

	return &exercise, nil
}

func dedupeIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func insertAllowedFileTypes(
	ctx context.Context, tx *sql.Tx, exerciseID int64, ids []int64,
) error {
	for _, id := range dedupeIDs(ids) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO exercises_allowed_file_types (exercise_id, allowed_file_type_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, exerciseID, id,
		); err != nil {
			slog.ErrorContext(ctx, "error inserting exercise allowed file type",
				slog.Int64("exercise_id", exerciseID),
				slog.Int64("allowed_file_type_id", id),
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
	}
	return nil
}

/*
ListOfferingExercises lists the exercises of an offering. Owners, admins and
co-professors see every exercise (including removed ones); monitors and enrolled
students only see the visible ones.
*/
func ListOfferingExercises(
	ctx context.Context, offeringID int64, claims map[string]any,
) ([]models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanView() {
		return nil, ErrOfferingNotFound
	}

	list, err := loadOfferingExercises(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	if rel.CanAuthor() {
		return list, nil
	}

	now := time.Now()
	visible := make([]models.Exercise, 0, len(list))
	for _, ex := range list {
		if exerciseVisible(ex, now) {
			visible = append(visible, ex)
		}
	}
	return visible, nil
}

/*
loadOfferingExercises returns the cached full exercise list of an offering (the
owner view, removed exercises included). Role-specific filtering is applied by
callers on top of it, so the cache key stays user-independent.
*/
func loadOfferingExercises(ctx context.Context, offeringID int64) ([]models.Exercise, error) {
	key := cache.OfferingExercisesKey(offeringID)

	var list []models.Exercise
	if cache.GetJSON(ctx, key, &list) {
		if list == nil {
			list = []models.Exercise{}
		}
		return list, nil
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, offering_id, title, description, deadline, open_date,
		       show_before_open_date, removed, created_at, updated_at
		FROM exercises
		WHERE offering_id = $1
		ORDER BY id`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching offering exercises",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	list = make([]models.Exercise, 0, 8)
	for rows.Next() {
		var ex models.Exercise
		if err := rows.Scan(
			&ex.ID, &ex.OfferingID, &ex.Title, &ex.Description, &ex.Deadline,
			&ex.OpenDate, &ex.ShowBeforeOpenDate, &ex.Removed,
			&ex.CreatedAt, &ex.UpdatedAt,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning offering exercise",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		ex.AllowedFileTypeIDs = []int64{}
		list = append(list, ex)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}

	if err := attachAllowedFileTypes(ctx, list); err != nil {
		return nil, err
	}

	cache.SetJSON(ctx, key, list, cache.ExercisesTTL)
	return list, nil
}

/*
attachAllowedFileTypes fills AllowedFileTypeIDs and AllowedFileTypes for a batch
of exercises from the catalog, in a single query.
*/
func attachAllowedFileTypes(ctx context.Context, exercises []models.Exercise) error {
	if len(exercises) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(exercises))
	index := make(map[int64]int, len(exercises))
	for i := range exercises {
		ids = append(ids, exercises[i].ID)
		index[exercises[i].ID] = i
		exercises[i].AllowedFileTypeIDs = []int64{}
		exercises[i].AllowedFileTypes = []models.AllowedFileType{}
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT eaft.exercise_id, aft.id, aft.name, aft.extension,
		       aft.is_compilable, aft.is_available
		FROM exercises_allowed_file_types eaft
		JOIN allowed_file_types aft ON aft.id = eaft.allowed_file_type_id
		WHERE eaft.exercise_id = ANY($1)
		ORDER BY eaft.exercise_id, aft.id`, pq.Array(ids))
	if err != nil {
		slog.ErrorContext(ctx, "error fetching exercises allowed file types",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var exerciseID int64
		var fileType models.AllowedFileType
		if err := rows.Scan(
			&exerciseID, &fileType.ID, &fileType.Name, &fileType.Extension,
			&fileType.IsCompilable, &fileType.IsAvailable,
		); err != nil {
			return ErrServer
		}
		if i, ok := index[exerciseID]; ok {
			exercises[i].AllowedFileTypeIDs = append(exercises[i].AllowedFileTypeIDs, fileType.ID)
			exercises[i].AllowedFileTypes = append(exercises[i].AllowedFileTypes, fileType)
		}
	}
	return rows.Err()
}

/*
GetExercise returns an exercise to its owner or to an enrolled student. Students
cannot see removed or not-yet-visible exercises.
*/
func GetExercise(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (*models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if !acc.canAuthor {
		if !acc.isEnrolled {
			return nil, ErrNotEnrolled
		}
		if !exerciseVisible(acc.exercise, time.Now()) {
			return nil, ErrExerciseNotFound
		}
	}

	exs := []models.Exercise{acc.exercise}
	if err := attachAllowedFileTypes(ctx, exs); err != nil {
		return nil, err
	}

	return &exs[0], nil
}

/*
UpdateExercise applies a partial update to an exercise owned by the caller.
*/
func UpdateExercise(
	ctx context.Context, exerciseID int64, req *models.UpdateExerciseRequest,
	claims map[string]any,
) (*models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if !acc.canAuthor {
		return nil, ErrNotOwner
	}

	ex := acc.exercise

	if req.Title != nil {
		ex.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		ex.Description = *req.Description
	}
	if req.Deadline != nil {
		ex.Deadline = *req.Deadline
	}
	if req.OpenDate != nil {
		ex.OpenDate = *req.OpenDate
	}
	if req.ShowBeforeOpenDate != nil {
		ex.ShowBeforeOpenDate = *req.ShowBeforeOpenDate
	}

	if err := ValidateExerciseFields(ex.Title, ex.Deadline, ex.OpenDate); err != nil {
		return nil, err
	}

	allowedProvided := req.AllowedFileTypeIDs != nil
	if allowedProvided {
		if err := validateAllowedFileTypeIDs(ctx, *req.AllowedFileTypeIDs); err != nil {
			return nil, err
		}
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing exercise update transaction",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		UPDATE exercises
		SET title = $1, description = $2, deadline = $3, open_date = $4,
		    show_before_open_date = $5, updated_at = now()
		WHERE id = $6
		RETURNING updated_at`,
		ex.Title, ex.Description, ex.Deadline, ex.OpenDate,
		ex.ShowBeforeOpenDate, exerciseID,
	).Scan(&ex.UpdatedAt)
	if err != nil {
		slog.ErrorContext(ctx, "error updating exercise",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if allowedProvided {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM exercises_allowed_file_types WHERE exercise_id = $1`,
			exerciseID,
		); err != nil {
			slog.ErrorContext(ctx, "error clearing exercise allowed file types",
				slog.Int64("exercise_id", exerciseID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		if err := insertAllowedFileTypes(ctx, tx, exerciseID, *req.AllowedFileTypeIDs); err != nil {
			return nil, err
		}
		ex.AllowedFileTypeIDs = dedupeIDs(*req.AllowedFileTypeIDs)
	} else {
		if ex.AllowedFileTypeIDs, err = loadAllowedFileTypeIDs(ctx, exerciseID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing exercise update",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.OfferingExercisesKey(ex.OfferingID))

	if exs := []models.Exercise{ex}; attachAllowedFileTypes(ctx, exs) == nil {
		ex = exs[0]
	}

	return &ex, nil
}

/*
DeleteExercise soft-deletes an exercise owned by the caller.
*/
func DeleteExercise(
	ctx context.Context, exerciseID int64, claims map[string]any,
) error {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return err
	}

	if _, err := database.DB.ExecContext(ctx, `
		UPDATE exercises
		SET removed = TRUE, updated_at = now()
		WHERE id = $1`, exerciseID,
	); err != nil {
		slog.ErrorContext(ctx, "error soft-deleting exercise",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	cache.Delete(ctx, cache.OfferingExercisesKey(acc.exercise.OfferingID))
	return nil
}

/*
ValidateExerciseFields validates the fields shared by create and update.
*/
func ValidateExerciseFields(title string, deadline, openDate time.Time) error {
	if err := validateTitle(title); err != nil {
		return err
	}
	if deadline.IsZero() || openDate.IsZero() {
		return validation.ErrRequiredField
	}
	if deadline.Before(openDate) {
		return ErrInvalidExercise
	}
	return nil
}

func validateTitle(title string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return validation.ErrRequiredField
	}
	if len([]rune(trimmed)) > MaxExerciseTitleLength {
		return validation.ErrInputTooLong
	}
	return nil
}
