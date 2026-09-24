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

// AdminListPageSize bounds one page of an admin listing.
const (
	DefaultAdminPageSize = 50
	MaxAdminPageSize     = 200
)

// userRoles are the values of the `user_t` enum.
var userRoles = map[string]struct{}{
	"student":   {},
	"professor": {},
	"admin":     {},
	"dev":       {},
}

// ValidUserRole reports whether role is one of the values of `user_t`.
func ValidUserRole(role string) bool {
	_, ok := userRoles[role]
	return ok
}

/*
normalizePage clamps the limit and offset an admin listing was asked for.
*/
func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultAdminPageSize
	}
	if limit > MaxAdminPageSize {
		limit = MaxAdminPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

/*
AdminListUsers lists the platform's users, optionally filtered by a search term
matched against the name, the email and the organization id.
*/
func AdminListUsers(
	ctx context.Context, query string, limit, offset int,
) ([]models.AdminUser, error) {
	limit, offset = normalizePage(limit, offset)
	pattern := "%" + strings.TrimSpace(query) + "%"

	rows, err := database.DB.QueryContext(ctx, `
		SELECT id, name, email, COALESCE(org_id, ''), role, confirmed, created_at
		FROM users
		WHERE ($1 = '%%' OR name ILIKE $1 OR email ILIKE $1
		       OR COALESCE(org_id, '') ILIKE $1)
		ORDER BY id
		LIMIT $2 OFFSET $3`, pattern, limit, offset,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error listing users",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	users := make([]models.AdminUser, 0, limit)
	for rows.Next() {
		var user models.AdminUser
		if err := rows.Scan(
			&user.ID, &user.Name, &user.Email, &user.OrgID, &user.Role,
			&user.Confirmed, &user.CreatedAt,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning user",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating users",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return users, nil
}

/*
AdminUpdateUser applies a partial update to any user of the platform. An admin
cannot change their own role: demoting the account that performs the change
would take the admin panel away from them mid-request.
*/
func AdminUpdateUser(
	ctx context.Context, userID int, req *models.UpdateUserRequest, adminID int,
) (*models.AdminUser, error) {
	if userID == adminID && req.Role != nil {
		return nil, ErrSelfManagement
	}

	var user models.AdminUser
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, email, COALESCE(org_id, ''), role, confirmed, created_at
		FROM users WHERE id = $1`, userID,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.OrgID, &user.Role,
		&user.Confirmed, &user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error fetching user",
			slog.String("error", err.Error()),
			slog.Int("user_id", userID),
		)
		return nil, ErrServer
	}

	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
		if err := validation.ValidateRequiredString(
			user.Name, MaxUserNameLength,
		); err != nil {
			return nil, err
		}
	}
	if req.Email != nil {
		user.Email = strings.TrimSpace(*req.Email)
		if err := validation.ValidateEmailFormat(ctx, user.Email); err != nil {
			return nil, err
		}
	}
	if req.OrgID != nil {
		user.OrgID = strings.TrimSpace(*req.OrgID)
		if err := validation.ValidateOptionalString(
			user.OrgID, MaxOrgIDLength,
		); err != nil {
			return nil, err
		}
	}
	if req.Role != nil {
		user.Role = strings.TrimSpace(*req.Role)
		if !ValidUserRole(user.Role) {
			return nil, ErrInvalidRole
		}
	}
	if req.Confirmed != nil {
		user.Confirmed = *req.Confirmed
	}

	_, err = database.DB.ExecContext(ctx, `
		UPDATE users
		SET name = $1, email = $2, org_id = NULLIF($3, ''), role = $4,
		    confirmed = $5, updated_at = now()
		WHERE id = $6`,
		user.Name, user.Email, user.OrgID, user.Role, user.Confirmed, user.ID,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok &&
			pgErr.Code == pqerror.UniqueViolation {
			switch pgErr.Constraint {
			case "users_email_key":
				return nil, ErrEmailExists
			case "users_org_id_key":
				return nil, ErrOrgIDExists
			}
		}
		slog.ErrorContext(ctx, "error updating user",
			slog.String("error", err.Error()),
			slog.Int("user_id", userID),
		)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "user updated by admin",
		slog.Int("user_id", userID),
		slog.Int("admin_id", adminID),
	)

	return &user, nil
}

/*
AdminDeleteUser removes a user and everything that hangs off them. The schema
cascades, so deleting a professor also deletes the classes they own and the
commits of every user in them; the objects those classes owned are collected
first and removed afterwards, exactly like an owner-facing delete.
*/
func AdminDeleteUser(ctx context.Context, userID, adminID int) error {
	if userID == adminID {
		return ErrSelfManagement
	}

	// Read before the delete: the keys live in rows that are about to go, and
	// the whole point is not to leave their objects behind.
	ownedIDs, err := ownedOfferingIDs(ctx, userID)
	if err != nil {
		return err
	}

	objects := make([]storageObject, 0, 8)
	for _, offeringID := range ownedIDs {
		keys, err := offeringStorageKeys(ctx, offeringID)
		if err != nil {
			return err
		}
		objects = append(objects, keys...)
	}

	result, err := database.DB.ExecContext(ctx,
		"DELETE FROM users WHERE id = $1", userID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting user",
			slog.String("error", err.Error()),
			slog.Int("user_id", userID),
		)
		return ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		slog.ErrorContext(ctx, "error reading user deletion result",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if affected == 0 {
		return ErrUserNotFound
	}

	for _, offeringID := range ownedIDs {
		cache.Delete(ctx, cache.OfferingKey(offeringID),
			cache.OfferingExercisesKey(offeringID))
	}
	deleteObjectsBestEffort(ctx, objects)

	slog.InfoContext(ctx, "user deleted by admin",
		slog.Int("user_id", userID),
		slog.Int("admin_id", adminID),
	)

	return nil
}

/*
ownedOfferingIDs lists the classes a user owns.
*/
func ownedOfferingIDs(ctx context.Context, userID int) ([]int64, error) {
	rows, err := database.DB.QueryContext(ctx,
		"SELECT id FROM offerings WHERE owner_id = $1", userID)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching owned offerings",
			slog.String("error", err.Error()),
			slog.Int("user_id", userID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	ids := make([]int64, 0, 4)
	for rows.Next() {
		var offeringID int64
		if err := rows.Scan(&offeringID); err != nil {
			slog.ErrorContext(ctx, "error scanning owned offering",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		ids = append(ids, offeringID)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating owned offerings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return ids, nil
}

/*
AdminListOfferings lists every class on the platform with its owner and size,
optionally filtered by a search term matched against the name and the owner.
*/
func AdminListOfferings(
	ctx context.Context, query string, limit, offset int,
) ([]models.AdminOffering, error) {
	limit, offset = normalizePage(limit, offset)
	pattern := "%" + strings.TrimSpace(query) + "%"

	return queryAdminOfferings(ctx, adminOfferingSearchFilter, pattern, limit, offset)
}

/*
AdminUpdateOffering edits any class on the platform, including the owner it is
transferred to. Authorization is the route's: only an admin reaches this.
*/
func AdminUpdateOffering(
	ctx context.Context, offeringID int64,
	req *models.AdminUpdateOfferingRequest,
) (*models.AdminOffering, error) {
	offering, ownerID, err := loadOffering(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	if req.OwnerID != nil {
		owner, err := loadOfferingOwner(ctx, *req.OwnerID)
		if err != nil {
			return nil, err
		}
		ownerID = sql.NullInt64{Int64: int64(owner.ID), Valid: true}
	}

	update := &models.UpdateOfferingRequest{
		Name:            req.Name,
		Description:     req.Description,
		VisibleToEnroll: req.VisibleToEnroll,
	}
	if req.EndDate != nil {
		formatted := req.EndDate.Format(time.RFC3339)
		update.EndDate = &formatted
	}

	rel := &offeringRelation{
		Offering:      *offering,
		OwnerID:       ownerID,
		AdminOverride: true,
	}

	if _, err := updateOfferingRow(ctx, rel, update); err != nil {
		return nil, err
	}

	if req.OwnerID != nil {
		if _, err := database.DB.ExecContext(ctx, `
			UPDATE offerings SET owner_id = $1, updated_at = now()
			WHERE id = $2`, *req.OwnerID, offeringID,
		); err != nil {
			slog.ErrorContext(ctx, "error transferring offering",
				slog.String("error", err.Error()),
				slog.Int64("offering_id", offeringID),
			)
			return nil, ErrServer
		}
		// The cached offering carries the owner, so a transfer must drop it.
		cache.Delete(ctx, cache.OfferingKey(offeringID))
	}

	return adminOfferingByID(ctx, offeringID)
}

/*
loadOfferingOwner checks that a user exists and can hold a class.
*/
func loadOfferingOwner(ctx context.Context, userID int64) (*models.User, error) {
	var user models.User
	err := database.DB.QueryRowContext(ctx,
		"SELECT id, name, email, role FROM users WHERE id = $1", userID,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error fetching offering owner",
			slog.String("error", err.Error()),
			slog.Int64("user_id", userID),
		)
		return nil, ErrServer
	}

	if !isPrivilegedRole(user.Role) {
		return nil, ErrInvalidRole
	}

	return &user, nil
}

/*
adminOfferingByID re-reads one class in the admin shape.
*/
func adminOfferingByID(
	ctx context.Context, offeringID int64,
) (*models.AdminOffering, error) {
	list, err := queryAdminOfferings(ctx, "WHERE o.id = $1", offeringID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrOfferingNotFound
	}
	return &list[0], nil
}

/*
AdminDeleteOffering removes any class on the platform, with the same cascade and
object cleanup as the owner-facing delete.
*/
func AdminDeleteOffering(ctx context.Context, offeringID int64) error {
	offering, ownerID, err := loadOffering(ctx, offeringID)
	if err != nil {
		return err
	}

	rel := &offeringRelation{
		Offering:      *offering,
		OwnerID:       ownerID,
		AdminOverride: true,
	}

	return deleteOfferingRow(ctx, rel)
}

/*
AdminOfferingMembers lists the members of any class, for the admin panel.
*/
func AdminOfferingMembers(
	ctx context.Context, offeringID int64,
) ([]models.OfferingMember, error) {
	if _, _, err := loadOffering(ctx, offeringID); err != nil {
		return nil, err
	}
	return listOfferingMembers(ctx, offeringID)
}

/*
adminOfferingSelect is the projection every admin offering listing returns.
*/
const adminOfferingSelect = `
	SELECT o.id, o.name, COALESCE(o.description, ''), o.end_date,
	       COALESCE(o.enrollment_code, ''), o.visible_to_enroll,
	       o.owner_id, COALESCE(owner.name, ''), COALESCE(owner.email, ''),
	       (SELECT count(*) FROM enrollments en WHERE en.offering_id = o.id),
	       (SELECT count(*) FROM exercises ex
	         WHERE ex.offering_id = o.id AND NOT ex.removed),
	       o.created_at
	FROM offerings o
	LEFT JOIN users owner ON owner.id = o.owner_id`

/*
adminOfferingSearchFilter narrows a listing to the classes whose name or owner
matches the search term. The pattern is the first query argument.
*/
const adminOfferingSearchFilter = `
	WHERE ($1 = '%%' OR o.name ILIKE $1 OR COALESCE(owner.name, '') ILIKE $1
	       OR COALESCE(owner.email, '') ILIKE $1)`

/*
queryAdminOfferings runs an admin offering listing. The filter is always one of
the constants above — never a caller-supplied string — so building the query
with concatenation cannot introduce injection.
*/
func queryAdminOfferings(
	ctx context.Context, filter string, args ...any,
) ([]models.AdminOffering, error) {
	rows, err := database.DB.QueryContext(ctx,
		adminOfferingSelect+"\n"+filter+"\nORDER BY o.id", args...)
	if err != nil {
		slog.ErrorContext(ctx, "error listing offerings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	offerings := make([]models.AdminOffering, 0, 8)
	for rows.Next() {
		var offering models.AdminOffering
		if err := rows.Scan(
			&offering.ID, &offering.Name, &offering.Description,
			&offering.EndDate, &offering.EnrollmentCode,
			&offering.VisibleToEnroll, &offering.OwnerID, &offering.OwnerName,
			&offering.OwnerEmail, &offering.MemberCount,
			&offering.ExerciseCount, &offering.CreatedAt,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning offering",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		offerings = append(offerings, offering)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating offerings",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return offerings, nil
}
