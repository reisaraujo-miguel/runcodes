package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
)

// Enrollment roles, as stored in the `enrollment_role_t` enum. A student joins
// with a code; the other two are assigned by the owner of the offering.
const (
	EnrollmentRoleStudent   = "student"
	EnrollmentRoleMonitor   = "monitor"
	EnrollmentRoleProfessor = "professor"
)

/*
ValidAssignableEnrollmentRole reports whether an offering owner may assign the
role through the member endpoints. 'student' is deliberately excluded: students
join with the enrollment code, not by being added to a class.
*/
func ValidAssignableEnrollmentRole(role string) bool {
	return role == EnrollmentRoleMonitor || role == EnrollmentRoleProfessor
}

/*
Enroll adds the requesting user to the offering that the given enrollment code
belongs to. The operation is idempotent: getting the code twice leaves the user
with a single enrollment.

Both the six-character codes generated for new offerings and the four-character
codes of older offerings are accepted: the lookup compares the upper-cased input
with the stored code, whatever its length.
*/
func Enroll(
	ctx context.Context, req *models.EnrollRequest, claims map[string]any,
) (*models.Enrollment, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	code := strings.ToUpper(strings.TrimSpace(req.EnrollmentCode))
	if code == "" {
		return nil, ErrInvalidEnrollmentCode
	}

	var (
		offering models.Enrollment
		ownerID  sql.NullInt64
		visible  bool
		role     string
		banned   bool
		enrolled bool
	)

	err = database.DB.QueryRowContext(ctx, `
		SELECT o.id, o.name, COALESCE(o.description, ''), o.end_date,
		       o.owner_id, o.visible_to_enroll,
		       COALESCE(en.role::text, ''), COALESCE(en.banned, FALSE),
		       (en.user_id IS NOT NULL), COALESCE(owner.name, '')
		FROM offerings o
		LEFT JOIN enrollments en
		       ON en.offering_id = o.id AND en.user_id = $2
		LEFT JOIN users owner ON owner.id = o.owner_id
		WHERE upper(o.enrollment_code) = $1`,
		code, userID,
	).Scan(
		&offering.OfferingID, &offering.Name, &offering.Description,
		&offering.EndDate, &ownerID, &visible, &role, &banned, &enrolled,
		&offering.OwnerName,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidEnrollmentCode
		}
		slog.ErrorContext(ctx, "error fetching offering by enrollment code",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if ownerID.Valid && ownerID.Int64 == userID {
		return nil, ErrOfferingOwned
	}
	if enrolled {
		if banned {
			return nil, ErrEnrollmentBanned
		}
		offering.Role = role
		return &offering, nil
	}
	if !visible {
		return nil, ErrEnrollmentClosed
	}
	if offering.EndDate.Before(time.Now()) {
		return nil, ErrOfferingEnded
	}

	if _, err := database.DB.ExecContext(ctx, `
		INSERT INTO enrollments (user_id, offering_id, role)
		VALUES ($1, $2, 'student'::enrollment_role_t)
		ON CONFLICT (user_id, offering_id) DO NOTHING`,
		userID, offering.OfferingID,
	); err != nil {
		slog.ErrorContext(ctx, "error inserting enrollment",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offering.OfferingID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	offering.Role = EnrollmentRoleStudent

	slog.InfoContext(ctx, "user enrolled in offering",
		slog.Int64("user_id", userID),
		slog.Int64("offering_id", offering.OfferingID),
	)

	return &offering, nil
}

/*
Unenroll removes the caller's own enrollment from an offering. The owner of an
offering cannot unenroll from it: they hold the class, and removing the
ownership is a separate operation. A banned enrollment is kept: the ban lives on
that row, so dropping it would let the student back in with the same code.
*/
func Unenroll(ctx context.Context, offeringID int64, claims map[string]any) error {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return err
	}

	var ownerID sql.NullInt64
	err = database.DB.QueryRowContext(ctx,
		"SELECT owner_id FROM offerings WHERE id = $1", offeringID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOfferingNotFound
		}
		slog.ErrorContext(ctx, "error fetching offering for unenroll",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if ownerID.Valid && ownerID.Int64 == userID {
		return ErrOwnerCannotUnenroll
	}

	result, err := database.DB.ExecContext(ctx,
		`DELETE FROM enrollments
		 WHERE user_id = $1 AND offering_id = $2 AND NOT banned`,
		userID, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting enrollment",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		slog.ErrorContext(ctx, "error reading unenroll result",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if affected == 0 {
		// The ban lives on the enrollment row, so deleting it would be a way for
		// a banned student to walk back in with the same enrollment code.
		var banned bool
		err := database.DB.QueryRowContext(ctx, `
			SELECT banned FROM enrollments
			WHERE user_id = $1 AND offering_id = $2`, userID, offeringID,
		).Scan(&banned)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "error checking enrollment after unenroll",
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
		if banned {
			return ErrEnrollmentBanned
		}
		return ErrNotEnrolled
	}

	slog.InfoContext(ctx, "user unenrolled from offering",
		slog.Int64("user_id", userID),
		slog.Int64("offering_id", offeringID),
	)

	return nil
}

/*
GetUserOffering returns one class the caller belongs to or owns, together with
the caller's progress on each exercise they can see: submission count, latest
status and best score. It backs the student-facing class page. The enrollment
code is deliberately absent, so an enrolled student cannot invite others.
*/
func GetUserOffering(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.UserOfferingDetail, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	// The LEFT JOIN is restricted to an active enrollment, and the WHERE clause
	// accepts either that enrollment or ownership. A banned member therefore
	// gets the same "not found" answer as a stranger, without disclosing that
	// the class exists.
	var detail models.UserOfferingDetail
	err = database.DB.QueryRowContext(ctx, `
		SELECT o.id, o.name, COALESCE(o.description, ''), o.end_date,
		       CASE WHEN o.owner_id = $1 THEN 'professor'
		            ELSE en.role::text END,
		       COALESCE(owner.name, ''),
		       COALESCE(o.owner_id = $1, FALSE),
		       (SELECT count(*) FROM enrollments members
		         WHERE members.offering_id = o.id)
		FROM offerings o
		LEFT JOIN enrollments en
		       ON en.offering_id = o.id AND en.user_id = $1 AND NOT en.banned
		LEFT JOIN users owner ON owner.id = o.owner_id
		WHERE o.id = $2 AND (en.user_id IS NOT NULL OR o.owner_id = $1)`,
		userID, offeringID,
	).Scan(
		&detail.ID, &detail.Name, &detail.Description, &detail.EndDate,
		&detail.Role, &detail.OwnerName, &detail.IsOwner, &detail.MemberCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOfferingNotFound
		}
		slog.ErrorContext(ctx, "error fetching user offering",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	detail.Exercises, err = listUserOfferingExercises(ctx, offeringID, userID)
	if err != nil {
		return nil, err
	}

	return &detail, nil
}

/*
listUserOfferingExercises reads the exercises a participant may see (not
removed, and either already open or explicitly shown before the open date) and
aggregates the caller's own commits for each one. The aggregate runs in a lateral
subquery so the whole overview is one query instead of one request per exercise.
*/
func listUserOfferingExercises(
	ctx context.Context, offeringID, userID int64,
) ([]models.UserOfferingExercise, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT e.id, e.offering_id, e.title, COALESCE(e.description, ''),
		       e.deadline, e.open_date,
		       progress.submission_count,
		       progress.last_submission_at,
		       progress.last_status,
		       progress.best_score
		FROM exercises e
		LEFT JOIN LATERAL (
			SELECT count(*) AS submission_count,
			       max(c.created_at) AS last_submission_at,
			       (array_agg(c.status::text
			                  ORDER BY c.created_at DESC, c.id DESC))[1]
			           AS last_status,
			       max(c.score) FILTER (
			           WHERE c.status NOT IN (
			               'pending', 'queued', 'compiling', 'running'
			           )
			       ) AS best_score
			FROM commits c
			WHERE c.exercise_id = e.id AND c.user_id = $2
		) progress ON TRUE
		WHERE e.offering_id = $1
		  AND NOT e.removed
		  AND (e.show_before_open_date OR e.open_date <= now())
		ORDER BY e.deadline, e.id`, offeringID, userID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching user offering exercises",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	exercises := make([]models.UserOfferingExercise, 0, 8)
	for rows.Next() {
		var (
			exercise         models.UserOfferingExercise
			lastSubmissionAt sql.NullTime
			lastStatus       sql.NullString
			bestScore        sql.NullFloat64
		)
		if err := rows.Scan(
			&exercise.ID, &exercise.OfferingID, &exercise.Title,
			&exercise.Description, &exercise.Deadline, &exercise.OpenDate,
			&exercise.SubmissionCount, &lastSubmissionAt, &lastStatus,
			&bestScore,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning user offering exercise",
				slog.Int64("user_id", userID),
				slog.Int64("offering_id", offeringID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		// Pointers distinguish "no submission yet" from a zero score/status, so
		// the client can show "Sem submissão" instead of "Nota 0".
		if lastSubmissionAt.Valid {
			submittedAt := lastSubmissionAt.Time
			exercise.LastSubmissionAt = &submittedAt
		}
		if lastStatus.Valid {
			status := lastStatus.String
			exercise.LastStatus = &status
		}
		if bestScore.Valid {
			score := bestScore.Float64
			exercise.BestScore = &score
		}

		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating user offering exercises",
			slog.Int64("user_id", userID),
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return exercises, nil
}

/*
ListUserOfferings lists the classes the caller belongs to: the ones they are
enrolled in (any role, not banned) and the ones they own. It backs the
"my classes" section of the home page.
*/
func ListUserOfferings(
	ctx context.Context, claims map[string]any,
) ([]models.Enrollment, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT o.id, o.name, COALESCE(o.description, ''), o.end_date,
		       CASE WHEN o.owner_id = $1 THEN 'professor'
		            ELSE en.role::text END,
		       COALESCE(owner.name, ''),
		       COALESCE(o.owner_id = $1, FALSE)
		FROM offerings o
		LEFT JOIN enrollments en
		       ON en.offering_id = o.id AND en.user_id = $1 AND NOT en.banned
		LEFT JOIN users owner ON owner.id = o.owner_id
		WHERE en.user_id IS NOT NULL OR o.owner_id = $1
		ORDER BY o.end_date DESC, o.name`, userID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching user offerings",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	offerings := make([]models.Enrollment, 0, 8)
	for rows.Next() {
		var offering models.Enrollment
		if err := rows.Scan(
			&offering.OfferingID, &offering.Name, &offering.Description,
			&offering.EndDate, &offering.Role, &offering.OwnerName,
			&offering.IsOwner,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning user offering",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		offerings = append(offerings, offering)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating user offerings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return offerings, nil
}

// MaxOpenExercises bounds the home page list so it stays a digest rather than a
// full history of every class the user ever joined.
const MaxOpenExercises = 50

/*
ListUserOpenExercises lists the exercises that are open right now in the
caller's classes (enrolled or owned): published, not removed, past their open
date and before their deadline.
*/
func ListUserOpenExercises(
	ctx context.Context, claims map[string]any,
) ([]models.OpenExercise, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT e.id, e.offering_id, o.name, e.title,
		       COALESCE(e.description, ''), e.deadline, e.open_date
		FROM exercises e
		JOIN offerings o ON o.id = e.offering_id
		LEFT JOIN enrollments en
		       ON en.offering_id = o.id AND en.user_id = $1 AND NOT en.banned
		WHERE e.removed = FALSE
		  AND e.open_date <= now()
		  AND e.deadline >= now()
		  AND (en.user_id IS NOT NULL OR o.owner_id = $1)
		ORDER BY e.deadline
		LIMIT $2`, userID, MaxOpenExercises,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching user open exercises",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	exercises := make([]models.OpenExercise, 0, 8)
	for rows.Next() {
		var exercise models.OpenExercise
		if err := rows.Scan(
			&exercise.ID, &exercise.OfferingID, &exercise.OfferingName,
			&exercise.Title, &exercise.Description, &exercise.Deadline,
			&exercise.OpenDate,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning user open exercise",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating user open exercises",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return exercises, nil
}
