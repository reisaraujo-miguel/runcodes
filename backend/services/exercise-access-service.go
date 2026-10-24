package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"

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
	if cache.GetJSON(ctx, cache.KeyAllowedFileTypes, &types) {
		return types, nil
	}

	rows, err := database.DB.QueryContext(ctx, `
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

	cache.SetJSON(ctx, cache.KeyAllowedFileTypes, types, cache.AllowedFileTypesTTL)
	return types, nil
}

/*
exerciseAccess is the loaded state of an exercise plus the caller's relation to
its offering.
*/
type exerciseAccess struct {
	exercise models.Exercise

	offeringOwnerID int64

	// canAuthor is true when the caller may change the exercise's authoring rows
	// (the owner, an admin, or a professor assigned to teach the class).
	// enrollmentRole is the caller's enrollment role ("" when not enrolled) and
	// banned whether that enrollment is banned.
	canAuthor      bool
	isEnrolled     bool
	enrollmentRole string
	banned         bool

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
loadExerciseAccess fetches the exercise together with the caller's relation to
its offering: whether they own it, teach it, or are enrolled in it.
*/
func loadExerciseAccess(
	ctx context.Context, exerciseID, userID int64, role string,
) (*exerciseAccess, error) {
	var (
		acc             exerciseAccess
		offeringOwnerID sql.NullInt64
	)

	err := database.DB.QueryRowContext(ctx, `
		SELECT e.id, e.offering_id, e.title, e.description, e.deadline,
		       e.open_date, e.show_before_open_date, e.removed,
		       e.created_at, e.updated_at, e.ghost, e.real_id, o.owner_id,
		       COALESCE(en.role::text, ''), COALESCE(en.banned, FALSE)
		FROM exercises e
		JOIN offerings o ON o.id = e.offering_id
		LEFT JOIN enrollments en
		       ON en.offering_id = e.offering_id AND en.user_id = $2
		WHERE e.id = $1`, exerciseID, userID,
	).Scan(
		&acc.exercise.ID, &acc.exercise.OfferingID, &acc.exercise.Title,
		&acc.exercise.Description, &acc.exercise.Deadline, &acc.exercise.OpenDate,
		&acc.exercise.ShowBeforeOpenDate, &acc.exercise.Removed,
		&acc.exercise.CreatedAt, &acc.exercise.UpdatedAt, &acc.ghost, &acc.realID,
		&offeringOwnerID, &acc.enrollmentRole, &acc.banned,
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

	if offeringOwnerID.Valid {
		acc.offeringOwnerID = offeringOwnerID.Int64
	}
	acc.isEnrolled = acc.enrollmentRole != "" && !acc.banned
	owner := isPrivilegedRole(role) && offeringOwnerID.Valid &&
		offeringOwnerID.Int64 == userID
	acc.canAuthor = owner ||
		(acc.isEnrolled && acc.enrollmentRole == EnrollmentRoleProfessor)

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
requireExerciseAuthor loads an exercise and returns ErrNotOwner unless the caller
may author it: the offering's owner (professor or admin) or a professor assigned
to teach the class. Students and monitors never pass, even when enrolled.
*/
func requireExerciseAuthor(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (*exerciseAccess, error) {
	acc, err := loadExerciseAccessForClaims(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}
	if !acc.canAuthor {
		return nil, ErrNotOwner
	}
	return acc, nil
}

/*
CanAuthorExercise reports whether the caller may author an exercise: the
offering's owner (professor or admin) or a professor assigned to teach the class.
It is the same boundary requireExerciseAuthor enforces, exposed for read paths
that must decide whether a privileged viewer may inspect another user's
submission.
*/
func CanAuthorExercise(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (bool, error) {
	acc, err := loadExerciseAccessForClaims(ctx, exerciseID, claims)
	if err != nil {
		return false, err
	}
	return acc.canAuthor, nil
}

func loadExerciseAccessForClaims(
	ctx context.Context, exerciseID int64, claims map[string]any,
) (*exerciseAccess, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	return loadExerciseAccess(ctx, exerciseID, userID, role)
}

/*
loadAllowedFileTypeIDs returns the allowed file type ids configured for an
exercise (empty when none).
*/
func loadAllowedFileTypeIDs(ctx context.Context, exerciseID int64) ([]int64, error) {
	rows, err := database.DB.QueryContext(ctx, `
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
	err := database.DB.QueryRowContext(ctx, `
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
