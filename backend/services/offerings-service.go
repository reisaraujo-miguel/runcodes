// Package services provides business logic services for the application.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

// MaxOfferingNameLength bounds a class name.
const MaxOfferingNameLength = 100

// enrollmentCodeAttempts bounds retries when a generated code collides with an
// existing one. With 36^6 values a single retry is already unlikely; the bound
// only keeps a broken random source from looping forever.
const enrollmentCodeAttempts = 5

// errEnrollmentCodeCollision tells CreateOffering to generate another code.
var errEnrollmentCodeCollision = errors.New("enrollment code collision")

/*
CreateOffering creates a new offering on the platform. The enrollment code is
generated with crypto/rand: it is the authorization secret students use to join
the class, so it must not be derivable from the sequential offering id.
*/
func CreateOffering(
	ctx context.Context, req *models.CreateOfferingRequest, claims map[string]any,
) (*models.Offering, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	for attempt := 1; attempt <= enrollmentCodeAttempts; attempt++ {
		enrollmentCode, err := newEnrollmentCode()
		if err != nil {
			slog.ErrorContext(ctx,
				"error generating enrollment code",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		offering, err := insertOffering(ctx, req, userID, enrollmentCode)
		if err == nil {
			cache.Delete(ctx, cache.OfferingKey(offering.ID))
			return offering, nil
		}
		if !errors.Is(err, errEnrollmentCodeCollision) {
			return nil, err
		}
	}

	slog.ErrorContext(ctx,
		"could not generate a unique enrollment code",
		slog.Int("attempts", enrollmentCodeAttempts),
		slog.Any("user_id", claims["id"]),
	)
	return nil, ErrServer
}

/*
insertOffering inserts the class row with its generated code. A unique violation
on enrollment_code is reported as errEnrollmentCodeCollision so the caller can
retry with a new code.
*/
func insertOffering(
	ctx context.Context, req *models.CreateOfferingRequest, userID int64, enrollmentCode string,
) (*models.Offering, error) {
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx,
			"error initializing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", userID),
		)
		return nil, ErrServer
	}

	defer tx.Rollback()

	var id int64
	if err := tx.QueryRowContext(ctx,
		`
		INSERT INTO offerings (name, owner_id, end_date, description, enrollment_code)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
		`, req.Name, userID, req.EndDate, req.Description, enrollmentCode,
	).Scan(&id); err != nil {
		if isEnrollmentCodeCollision(err) {
			return nil, errEnrollmentCodeCollision
		}
		slog.ErrorContext(ctx,
			"error inserting new offering on the database",
			slog.String("error", err.Error()),
			slog.Any("user_id", userID),
		)
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx,
			"error committing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", userID),
		)
		return nil, ErrServer
	}

	return &models.Offering{
		ID:              id,
		Name:            req.Name,
		EndDate:         req.EndDate,
		Description:     req.Description,
		EnrollmentCode:  enrollmentCode,
		VisibleToEnroll: true,
	}, nil
}

/*
cachedOffering couples the cached offering with its owner so a cache hit can
still enforce ownership.
*/
type cachedOffering struct {
	Offering models.Offering `json:"offering"`
	OwnerID  sql.NullInt64   `json:"owner_id"`
}

/*
offeringRelation is a loaded offering plus the caller's relation to it. It is
the single place where "may this user touch this class?" is answered, so the
authoring endpoints, the member endpoints and the admin override cannot drift
apart.
*/
type offeringRelation struct {
	Offering models.Offering
	OwnerID  sql.NullInt64

	// Owner is true when the caller owns the offering. AdminOverride is set by
	// the admin panel, which manages classes it does not own.
	Owner         bool
	AdminOverride bool

	// Role is the caller's enrollment role ("" when not enrolled) and Banned
	// whether that enrollment is banned.
	Role   string
	Banned bool
}

// IsMember reports whether the caller is a non-banned participant of the class.
func (rel *offeringRelation) IsMember() bool {
	return rel.Role != "" && !rel.Banned
}

/*
CanView reports whether the caller may see the class and its exercises, including
the ones hidden from students.
*/
func (rel *offeringRelation) CanView() bool {
	return rel.Owner || rel.AdminOverride || rel.IsMember()
}

/*
CanAuthor reports whether the caller may change the class: its exercises, test
cases and files. That is the owner, the admin panel, or a professor the owner
assigned to teach the class. Students and monitors read and submit; they do not
author.
*/
func (rel *offeringRelation) CanAuthor() bool {
	return rel.Owner || rel.AdminOverride ||
		(rel.IsMember() && rel.Role == EnrollmentRoleProfessor)
}

/*
loadOffering fetches an offering by id, with its owner. The cache entry is user
independent: ownership is checked by the callers on top of it.
*/
func loadOffering(
	ctx context.Context, offeringID int64,
) (*models.Offering, sql.NullInt64, error) {
	key := cache.OfferingKey(offeringID)

	var cached cachedOffering
	if cache.GetJSON(ctx, key, &cached) {
		return &cached.Offering, cached.OwnerID, nil
	}

	var (
		offering models.Offering
		ownerID  sql.NullInt64
		endDate  time.Time
	)
	// description and enrollment_code are nullable: a class migrated from the old
	// system has neither. They are coalesced rather than scanned into sql.NullString
	// so the rest of the code keeps working with plain strings.
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, end_date, COALESCE(description, ''),
		       COALESCE(enrollment_code, ''), owner_id, visible_to_enroll
		FROM offerings
		WHERE id = $1`, offeringID,
	).Scan(
		&offering.ID, &offering.Name, &endDate, &offering.Description,
		&offering.EnrollmentCode, &ownerID, &offering.VisibleToEnroll,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ownerID, ErrOfferingNotFound
		}
		slog.ErrorContext(ctx, "error fetching offering from the database",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ownerID, ErrServer
	}
	offering.EndDate = endDate.Format(time.RFC3339)

	cache.SetJSON(ctx, key, cachedOffering{Offering: offering, OwnerID: ownerID},
		cache.OfferingTTL)

	return &offering, ownerID, nil
}

/*
loadOfferingRelation loads an offering and the caller's relation to it, checking
the caller's enrollment only when they do not own it.
*/
func loadOfferingRelation(
	ctx context.Context, offeringID, userID int64, role string,
) (*offeringRelation, error) {
	offering, ownerID, err := loadOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	rel := &offeringRelation{
		Offering: *offering,
		OwnerID:  ownerID,
		Owner:    isPrivilegedRole(role) && ownerID.Valid && ownerID.Int64 == userID,
	}
	if rel.Owner {
		return rel, nil
	}

	err = database.DB.QueryRowContext(ctx, `
		SELECT COALESCE(role::text, ''), COALESCE(banned, FALSE)
		FROM enrollments
		WHERE offering_id = $1 AND user_id = $2`, offeringID, userID,
	).Scan(&rel.Role, &rel.Banned)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.ErrorContext(ctx, "error checking offering membership",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
			slog.Int64("user_id", userID),
		)
		return nil, ErrServer
	}

	return rel, nil
}

/*
requireOfferingOwner loads an offering and returns ErrNotOwner unless the caller
owns it. It is the authorization boundary of the class management endpoints.
*/
func requireOfferingOwner(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*offeringRelation, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.Owner {
		return nil, ErrNotOwner
	}
	return rel, nil
}

/*
GetOffering fetches an offering for its owner or for a professor assigned to
teach it. The enrollment code is part of the response, so a student who is
merely enrolled cannot read it.
*/
func GetOffering(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.Offering, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		return nil, ErrOfferingNotFound
	}

	return &rel.Offering, nil
}

/*
ListOwnedOfferings lists the classes the caller manages: the ones they own and,
for a professor, the ones they were assigned to teach. It backs the "manage
classes" page.
*/
func ListOwnedOfferings(
	ctx context.Context, claims map[string]any,
) ([]models.OwnedOffering, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}
	if !isPrivilegedRole(role) {
		return nil, ErrNotOwner
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT o.id, o.name, o.end_date, COALESCE(o.description, ''),
		       COALESCE(o.enrollment_code, ''), o.visible_to_enroll,
		       COALESCE(o.owner_id, 0), COALESCE(o.owner_id = $1, FALSE),
		       (SELECT count(*) FROM enrollments en WHERE en.offering_id = o.id),
		       (SELECT count(*) FROM exercises ex
		         WHERE ex.offering_id = o.id AND NOT ex.removed)
		FROM offerings o
		LEFT JOIN enrollments en
		       ON en.offering_id = o.id AND en.user_id = $1
		      AND NOT en.banned AND en.role = 'professor'
		WHERE o.owner_id = $1 OR en.user_id IS NOT NULL
		ORDER BY o.end_date DESC, o.name`, userID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching managed offerings",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	list := make([]models.OwnedOffering, 0, 8)
	for rows.Next() {
		var offering models.OwnedOffering
		if err := rows.Scan(
			&offering.ID, &offering.Name, &offering.EndDate,
			&offering.Description, &offering.EnrollmentCode,
			&offering.VisibleToEnroll, &offering.OwnerID, &offering.IsOwner,
			&offering.MemberCount, &offering.ExerciseCount,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning managed offering",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		offering.EnrollmentOpen = offering.VisibleToEnroll &&
			offering.EndDate.After(time.Now())
		list = append(list, offering)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating managed offerings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return list, nil
}

/*
UpdateOffering applies a partial update to an offering owned by the caller.
*/
func UpdateOffering(
	ctx context.Context, offeringID int64, req *models.UpdateOfferingRequest,
	claims map[string]any,
) (*models.Offering, error) {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return nil, err
	}

	return updateOfferingRow(ctx, rel, req)
}

/*
updateOfferingRow writes an offering update. Authorization is the caller's:
UpdateOffering enforces ownership, the admin panel overrides it.
*/
func updateOfferingRow(
	ctx context.Context, rel *offeringRelation, req *models.UpdateOfferingRequest,
) (*models.Offering, error) {
	offering, err := updateOfferingFields(ctx, database.DB, rel, req)
	if err != nil {
		return nil, err
	}

	cache.Delete(ctx, cache.OfferingKey(offering.ID))

	slog.InfoContext(ctx, "offering updated",
		slog.Int64("offering_id", offering.ID),
	)

	return offering, nil
}

/*
RotateOfferingEnrollmentCode generates a new six-character code for a class. The
old code stops working immediately; this is the recovery path for classes whose
legacy code could not be rotated by the migration (or whose code is null). Only
the owner may rotate it; a co-professor can see the code but cannot invalidate
what the owner distributed.
*/
func RotateOfferingEnrollmentCode(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.Offering, error) {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return nil, err
	}

	for attempt := 1; attempt <= enrollmentCodeAttempts; attempt++ {
		code, err := newEnrollmentCode()
		if err != nil {
			slog.ErrorContext(ctx, "error generating enrollment code",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		_, err = database.DB.ExecContext(ctx, `
			UPDATE offerings
			SET enrollment_code = $1, updated_at = now()
			WHERE id = $2`, code, offeringID,
		)
		if err == nil {
			rel.Offering.EnrollmentCode = code
			cache.Delete(ctx, cache.OfferingKey(offeringID))
			slog.InfoContext(ctx, "offering enrollment code rotated",
				slog.Int64("offering_id", offeringID),
			)
			return &rel.Offering, nil
		}
		if !isEnrollmentCodeCollision(err) {
			slog.ErrorContext(ctx, "error rotating enrollment code",
				slog.Int64("offering_id", offeringID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	}

	slog.ErrorContext(ctx, "could not generate a unique enrollment code",
		slog.Int("attempts", enrollmentCodeAttempts),
		slog.Int64("offering_id", offeringID),
	)
	return nil, ErrServer
}

// isEnrollmentCodeCollision reports whether err is the unique violation on the
// enrollment-code column.
func isEnrollmentCodeCollision(err error) bool {
	pgErr, ok := err.(*pq.Error)
	return ok && pgErr.Code == pqerror.UniqueViolation &&
		pgErr.Constraint == "offerings_enrollment_code_key"
}

// offeringExecer is the subset of *sql.DB and *sql.Tx the field update needs, so
// the admin panel can run the field write and the ownership transfer in one
// transaction.
type offeringExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

/*
updateOfferingFields validates and writes the mutable offering fields. It does
not invalidate the cache or log success: the caller does that after the
surrounding transaction commits.
*/
func updateOfferingFields(
	ctx context.Context, exec offeringExecer,
	rel *offeringRelation, req *models.UpdateOfferingRequest,
) (*models.Offering, error) {
	offering := rel.Offering

	if req.Name != nil {
		offering.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		offering.Description = *req.Description
	}
	if req.EndDate != nil {
		offering.EndDate = strings.TrimSpace(*req.EndDate)
	}
	if req.VisibleToEnroll != nil {
		offering.VisibleToEnroll = *req.VisibleToEnroll
	}

	if err := validation.ValidateRequiredString(
		offering.Name, MaxOfferingNameLength,
	); err != nil {
		return nil, err
	}
	endDate, err := validation.ValidateDate(ctx, offering.EndDate)
	if err != nil {
		return nil, err
	}

	if _, err := exec.ExecContext(ctx, `
		UPDATE offerings
		SET name = $1, description = $2, end_date = $3,
		    visible_to_enroll = $4, updated_at = now()
		WHERE id = $5`,
		offering.Name, offering.Description, endDate, offering.VisibleToEnroll,
		offering.ID,
	); err != nil {
		slog.ErrorContext(ctx, "error updating offering",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offering.ID),
		)
		return nil, ErrServer
	}

	// The stored value comes back normalised, so report what a reload will show.
	offering.EndDate = endDate.Format(time.RFC3339)

	return &offering, nil
}

/*
DeleteOffering removes an offering owned by the caller, together with its
enrollments and exercises (the schema cascades both).
*/
func DeleteOffering(ctx context.Context, offeringID int64, claims map[string]any) error {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return err
	}

	return deleteOfferingRow(ctx, rel)
}

/*
deleteOfferingRow deletes an offering and cleans up the objects it owned. The
rows go first: an object left behind by a failed cleanup is orphaned storage,
while a row left behind by a failed delete is a class the owner cannot remove.
*/
func deleteOfferingRow(ctx context.Context, rel *offeringRelation) error {
	keys, err := offeringStorageKeys(ctx, rel.Offering.ID)
	if err != nil {
		return err
	}

	if _, err := database.DB.ExecContext(ctx, `
		DELETE FROM offerings
		WHERE id = $1`, rel.Offering.ID,
	); err != nil {
		slog.ErrorContext(ctx, "error deleting offering",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", rel.Offering.ID),
		)
		return ErrServer
	}

	cache.Delete(ctx, cache.OfferingKey(rel.Offering.ID),
		cache.OfferingExercisesKey(rel.Offering.ID))

	deleteObjectsBestEffort(ctx, keys)

	slog.InfoContext(ctx, "offering deleted",
		slog.Int64("offering_id", rel.Offering.ID),
	)

	return nil
}

/*
storageBucket identifies the S3 bucket that holds a storageObject. The keys of
the different kinds overlap in shape (a case id and an exercise id are both
numbers), so the bucket travels with the key instead of being re-derived from
it. In particular, submitted sources live in the commits bucket, not the files
bucket.
*/
