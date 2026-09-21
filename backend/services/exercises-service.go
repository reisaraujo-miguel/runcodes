package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
)

// MaxExerciseTitleLength bounds an exercise title.
const MaxExerciseTitleLength = 200

/*
claimsUserID extracts the numeric user id and role from JWT claims. Handlers
verify the claims shape first; a malformed claim here is an internal error.
*/
func claimsUserID(claims map[string]any) (int64, string, error) {
	idRaw, ok := claims["id"]
	if !ok {
		slog.Error("missing user id claim")
		return 0, "", ErrServer
	}
	id, ok := idRaw.(float64)
	if !ok {
		slog.Error("invalid user id claim type")
		return 0, "", ErrServer
	}

	role, _ := claims["role"].(string)
	return int64(id), role, nil
}

func isPrivilegedRole(role string) bool {
	return role == "professor" || role == "admin"
}

/*
ListAllowedFileTypes returns the available entries of the allowed file type
catalog. The catalog is global and rarely changes, so it is cached.
*/
func ListAllowedFileTypes(ctx context.Context) ([]models.AllowedFileType, error) {
	var types []models.AllowedFileType
	if CacheGetJSON(ctx, cacheKeyAllowedFileTypes, &types) {
		return types, nil
	}

	rows, err := DB.QueryContext(ctx, `
		SELECT id, name, extension, is_compilable, is_available
		FROM allowed_file_types
		WHERE is_available
		ORDER BY id`)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching allowed file types",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	types = make([]models.AllowedFileType, 0, 16)
	for rows.Next() {
		var t models.AllowedFileType
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Extension, &t.IsCompilable, &t.IsAvailable,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning allowed file type",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		types = append(types, t)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating allowed file types",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	CacheSetJSON(ctx, cacheKeyAllowedFileTypes, types, CacheAllowedFileTypesTTL)
	return types, nil
}

/*
offeringAccess reports whether the user owns the offering (privileged role) or
is an enrolled, non-banned participant.
*/
func offeringAccess(
	ctx context.Context, offeringID, userID int64, role string,
) (owner bool, enrolled bool, err error) {
	var (
		ownerID    int64
		isEnrolled bool
	)

	err = DB.QueryRowContext(ctx, `
		SELECT o.owner_id,
		       EXISTS (
		           SELECT 1 FROM enrollments en
		           WHERE en.offering_id = o.id AND en.user_id = $2 AND NOT en.banned
		       )
		FROM offerings o
		WHERE o.id = $1`, offeringID, userID,
	).Scan(&ownerID, &isEnrolled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, false, ErrOfferingNotFound
		}
		slog.ErrorContext(ctx, "error checking offering access",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return false, false, ErrServer
	}

	owner = isPrivilegedRole(role) && ownerID == userID
	return owner, isEnrolled, nil
}

/*
exerciseAccess is the loaded state of an exercise plus the caller's relation to
its offering.
*/
type exerciseAccess struct {
	exercise models.Exercise

	offeringOwnerID int64
	isOwner         bool
	isEnrolled      bool

	// ghost exercises reuse the test cases and compilation files of their
	// real exercise, exactly like the judge does.
	ghost  bool
	realID sql.NullInt64
}

/*
authoringExerciseID is the exercise id that owns authoring rows (test cases and
compilation files). For a ghost exercise this is its real exercise.
*/
func (a *exerciseAccess) authoringExerciseID() int64 {
	if a.ghost && a.realID.Valid {
		return a.realID.Int64
	}
	return a.exercise.ID
}

/*
loadExerciseAccess fetches the exercise together with whether the caller owns
the offering or is enrolled in it.
*/
func loadExerciseAccess(
	ctx context.Context, exerciseID, userID int64, role string,
) (*exerciseAccess, error) {
	var (
		acc      exerciseAccess
		enrolled bool
	)

	err := DB.QueryRowContext(ctx, `
		SELECT e.id, e.offering_id, e.title, e.description, e.deadline,
		       e.open_date, e.show_before_open_date, e.removed,
		       e.created_at, e.updated_at, e.ghost, e.real_id, o.owner_id,
		       EXISTS (
		           SELECT 1 FROM enrollments en
		           WHERE en.offering_id = e.offering_id AND en.user_id = $2
		             AND NOT en.banned
		       )
		FROM exercises e
		JOIN offerings o ON o.id = e.offering_id
		WHERE e.id = $1`, exerciseID, userID,
	).Scan(
		&acc.exercise.ID, &acc.exercise.OfferingID, &acc.exercise.Title,
		&acc.exercise.Description, &acc.exercise.Deadline, &acc.exercise.OpenDate,
		&acc.exercise.ShowBeforeOpenDate, &acc.exercise.Removed,
		&acc.exercise.CreatedAt, &acc.exercise.UpdatedAt, &acc.ghost, &acc.realID,
		&acc.offeringOwnerID, &enrolled,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExerciseNotFound
		}
		slog.ErrorContext(ctx, "error fetching exercise",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	acc.isEnrolled = enrolled
	acc.isOwner = isPrivilegedRole(role) && acc.offeringOwnerID == userID
	return &acc, nil
}

/*
exerciseVisible reports whether a student may see an exercise: it must not be
removed and must either be past its open date or explicitly shown before it.
*/
func exerciseVisible(ex models.Exercise, now time.Time) bool {
	if ex.Removed {
		return false
	}
	return ex.ShowBeforeOpenDate || !ex.OpenDate.After(now)
}

/*
requireExerciseOwner loads an exercise and returns ErrNotOwner unless the caller
owns the offering it belongs to.
*/
func requireExerciseOwner(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (*exerciseAccess, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	acc, err := loadExerciseAccess(ctx, exerciseID, userID, role)
	if err != nil {
		return nil, err
	}
	if !acc.isOwner {
		return nil, ErrNotOwner
	}
	return acc, nil
}

/*
loadAllowedFileTypeIDs returns the allowed file type ids configured for an
exercise (empty when none).
*/
func loadAllowedFileTypeIDs(ctx context.Context, exerciseID int64) ([]int64, error) {
	rows, err := DB.QueryContext(ctx, `
		SELECT allowed_file_type_id
		FROM exercises_allowed_file_types
		WHERE exercise_id = $1
		ORDER BY allowed_file_type_id`, exerciseID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching exercise allowed file types",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	ids := make([]int64, 0, 4)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			slog.ErrorContext(ctx, "error scanning exercise allowed file type",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return ids, nil
}

/*
validateAllowedFileTypeIDs rejects ids that do not exist or are unavailable.
*/
func validateAllowedFileTypeIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	var count int
	err := DB.QueryRowContext(ctx, `
		SELECT count(*) FROM allowed_file_types
		WHERE id = ANY($1) AND is_available`, pq.Array(unique),
	).Scan(&count)
	if err != nil {
		slog.ErrorContext(ctx, "error validating allowed file type ids",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if count != len(unique) {
		return ErrInvalidFileType
	}
	return nil
}

/*
CreateExercise creates an exercise in an offering owned by the caller.
*/
func CreateExercise(
	ctx context.Context, offeringID int64, req *models.CreateExerciseRequest,
	claims map[string]any,
) (*models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	owner, _, err := offeringAccess(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !owner {
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

	tx, err := DB.BeginTx(ctx, nil)
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
	CacheDelete(ctx, cacheKeyOfferingExercises(offeringID))

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
ListOfferingExercises lists the exercises of an offering. Owners see every
exercise (including removed ones); enrolled students only see visible ones.
*/
func ListOfferingExercises(
	ctx context.Context, offeringID int64, claims map[string]any,
) ([]models.Exercise, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	owner, enrolled, err := offeringAccess(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !owner && !enrolled {
		return nil, ErrOfferingNotFound
	}

	list, err := loadOfferingExercises(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	if owner {
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
	key := cacheKeyOfferingExercises(offeringID)

	var list []models.Exercise
	if CacheGetJSON(ctx, key, &list) {
		if list == nil {
			list = []models.Exercise{}
		}
		return list, nil
	}

	rows, err := DB.QueryContext(ctx, `
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

	CacheSetJSON(ctx, key, list, CacheExercisesTTL)
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

	rows, err := DB.QueryContext(ctx, `
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
	if !acc.isOwner {
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
	if !acc.isOwner {
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

	tx, err := DB.BeginTx(ctx, nil)
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

	CacheDelete(ctx, cacheKeyOfferingExercises(ex.OfferingID))

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
	acc, err := requireExerciseOwner(ctx, exerciseID, claims)
	if err != nil {
		return err
	}

	if _, err := DB.ExecContext(ctx, `
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

	CacheDelete(ctx, cacheKeyOfferingExercises(acc.exercise.OfferingID))
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
