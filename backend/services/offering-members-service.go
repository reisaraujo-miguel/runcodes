package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"
)

func ListOfferingMembers(
	ctx context.Context, offeringID int64, claims map[string]any,
) ([]models.OfferingMember, error) {
	if _, err := requireOfferingOwner(ctx, offeringID, claims); err != nil {
		return nil, err
	}

	return listOfferingMembers(ctx, offeringID)
}

/*
listOfferingMembers reads the members of an offering. Authorization is the
caller's.
*/
func listOfferingMembers(
	ctx context.Context, offeringID int64,
) ([]models.OfferingMember, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT u.id, u.name, u.email, en.role::text, en.banned, en.created_at
		FROM enrollments en
		JOIN users u ON u.id = en.user_id
		WHERE en.offering_id = $1
		ORDER BY en.role, u.name`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error fetching offering members",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	members := make([]models.OfferingMember, 0, 8)
	for rows.Next() {
		var member models.OfferingMember
		if err := rows.Scan(
			&member.UserID, &member.Name, &member.Email, &member.Role,
			&member.Banned, &member.CreatedAt,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning offering member",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating offering members",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return members, nil
}

/*
AddOfferingMember adds a monitor or a co-professor to an offering owned by the
caller, looked up by email. A student who is already enrolled is promoted in
place rather than added twice.
*/
func AddOfferingMember(
	ctx context.Context, offeringID int64, req *models.AddMemberRequest,
	claims map[string]any,
) (*models.OfferingMember, error) {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return nil, err
	}

	return addOfferingMemberRow(ctx, rel, req)
}

/*
addOfferingMemberRow enrolls a user in an offering with an assigned role. A
student who is already enrolled is promoted in place rather than added twice.
*/
func addOfferingMemberRow(
	ctx context.Context, rel *offeringRelation, req *models.AddMemberRequest,
) (*models.OfferingMember, error) {
	role := strings.TrimSpace(req.Role)
	if !ValidAssignableEnrollmentRole(role) {
		return nil, ErrInvalidMemberRole
	}

	email := validation.NormalizeEmail(req.Email)
	if err := validation.ValidateEmailFormat(ctx, email); err != nil {
		return nil, err
	}

	var member models.OfferingMember
	var platformRole string
	err := database.DB.QueryRowContext(ctx,
		"SELECT id, name, email, role::text FROM users WHERE email = $1", email,
	).Scan(&member.UserID, &member.Name, &member.Email, &platformRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error fetching member by email",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if rel.OwnerID.Valid && rel.OwnerID.Int64 == member.UserID {
		return nil, ErrAlreadyMember
	}

	if err := requireTeachingRole(role, platformRole); err != nil {
		return nil, err
	}

	// The response is the stored row, not the request: the upsert may have
	// promoted an existing enrollment, and created_at only exists there.
	err = database.DB.QueryRowContext(ctx, `
		INSERT INTO enrollments (user_id, offering_id, role)
		VALUES ($1, $2, $3::enrollment_role_t)
		ON CONFLICT (user_id, offering_id) DO UPDATE
		SET role = EXCLUDED.role, banned = FALSE, updated_at = now()
		RETURNING role::text, banned, created_at`,
		member.UserID, rel.Offering.ID, role,
	).Scan(&member.Role, &member.Banned, &member.CreatedAt)
	if err != nil {
		slog.ErrorContext(ctx, "error inserting offering member",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", rel.Offering.ID),
			slog.Int64("user_id", member.UserID),
		)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "offering member added",
		slog.Int64("offering_id", rel.Offering.ID),
		slog.Int64("user_id", member.UserID),
		slog.String("role", role),
	)

	return &member, nil
}

/*
requireTeachingRole refuses to make somebody a co-professor of a class when their
account is not a professor: the authoring endpoints decide access by the
enrollment role, so the grant would hand out rights the account cannot exercise
anywhere else in the UI.
*/
func requireTeachingRole(enrollmentRole, platformRole string) error {
	if enrollmentRole != EnrollmentRoleProfessor {
		return nil
	}
	if !isPrivilegedRole(platformRole) {
		return ErrNotAProfessor
	}
	return nil
}

/*
UpdateOfferingMember changes the role of a member of an offering owned by the
caller, or bans and unbans them.
*/
func UpdateOfferingMember(
	ctx context.Context, offeringID, memberID int64,
	req *models.UpdateMemberRequest, claims map[string]any,
) (*models.OfferingMember, error) {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return nil, err
	}

	return updateOfferingMemberRow(ctx, rel, memberID, req)
}

/*
updateOfferingMemberRow rewrites one enrollment of an offering. A demoted
professor keeps their enrollment and becomes a monitor. Banning is independent
of the role: a banned member keeps it but loses access, because every access
check goes through IsMember.
*/
func updateOfferingMemberRow(
	ctx context.Context, rel *offeringRelation, memberID int64,
	req *models.UpdateMemberRequest,
) (*models.OfferingMember, error) {
	if rel.OwnerID.Valid && rel.OwnerID.Int64 == memberID {
		return nil, ErrCannotBanOwner
	}

	var role string
	if req.Role != nil {
		role = strings.TrimSpace(*req.Role)
		if !ValidAssignableEnrollmentRole(role) {
			return nil, ErrInvalidMemberRole
		}

		memberPlatformRole, err := platformRoleOf(ctx, memberID)
		if err != nil {
			return nil, err
		}
		if err := requireTeachingRole(role, memberPlatformRole); err != nil {
			return nil, err
		}
	}

	var banned sql.NullBool
	if req.Banned != nil {
		banned = sql.NullBool{Valid: true, Bool: *req.Banned}
	}

	var member models.OfferingMember
	err := database.DB.QueryRowContext(ctx, `
		UPDATE enrollments en
		SET role = COALESCE(NULLIF($3::text, '')::enrollment_role_t, en.role),
		    banned = COALESCE($4, en.banned),
		    updated_at = now()
		FROM users u
		WHERE en.offering_id = $1 AND en.user_id = $2 AND u.id = en.user_id
		RETURNING u.id, u.name, u.email, en.role::text, en.banned, en.created_at`,
		rel.Offering.ID, memberID, role, banned,
	).Scan(
		&member.UserID, &member.Name, &member.Email, &member.Role,
		&member.Banned, &member.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		slog.ErrorContext(ctx, "error updating offering member",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", rel.Offering.ID),
			slog.Int64("user_id", memberID),
		)
		return nil, ErrServer
	}

	return &member, nil
}

/*
platformRoleOf returns a user's platform role.
*/
func platformRoleOf(ctx context.Context, userID int64) (string, error) {
	var role string
	err := database.DB.QueryRowContext(ctx,
		"SELECT role::text FROM users WHERE id = $1", userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrMemberNotFound
		}
		slog.ErrorContext(ctx, "error fetching user role",
			slog.String("error", err.Error()),
			slog.Int64("user_id", userID),
		)
		return "", ErrServer
	}
	return role, nil
}

/*
RemoveOfferingMember drops a member from an offering owned by the caller. The
owner cannot be removed from their own class.
*/
func RemoveOfferingMember(
	ctx context.Context, offeringID, memberID int64, claims map[string]any,
) error {
	rel, err := requireOfferingOwner(ctx, offeringID, claims)
	if err != nil {
		return err
	}

	return removeOfferingMemberRow(ctx, rel, memberID)
}

/*
removeOfferingMemberRow deletes one enrollment of an offering.
*/
func removeOfferingMemberRow(
	ctx context.Context, rel *offeringRelation, memberID int64,
) error {
	if rel.OwnerID.Valid && rel.OwnerID.Int64 == memberID {
		return ErrCannotBanOwner
	}

	result, err := database.DB.ExecContext(ctx, `
		DELETE FROM enrollments
		WHERE offering_id = $1 AND user_id = $2`,
		rel.Offering.ID, memberID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error deleting offering member",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", rel.Offering.ID),
			slog.Int64("user_id", memberID),
		)
		return ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		slog.ErrorContext(ctx, "error reading member deletion result",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	if affected == 0 {
		return ErrMemberNotFound
	}

	return nil
}
