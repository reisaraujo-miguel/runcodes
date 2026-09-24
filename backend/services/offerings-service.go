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
	"github.com/runcodes-icmc/runcodes/storage"
	"github.com/runcodes-icmc/runcodes/validation"
)

// MaxOfferingNameLength bounds a class name.
const MaxOfferingNameLength = 100

/*
CreateOffering creates a new offering on the platform.
*/
func CreateOffering(
	ctx context.Context, req *models.CreateOfferingRequest, claims map[string]any,
) (*models.Offering, error) {
	userID, _, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	var tx *sql.Tx
	if tx, err = database.DB.BeginTx(ctx, nil); err != nil {
		slog.ErrorContext(ctx,
			"error initializing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return nil, ErrServer
	}

	defer tx.Rollback()

	var id int64
	if err = tx.QueryRowContext(ctx,
		`
		INSERT INTO offerings (name, owner_id, end_date, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id
		`, req.Name, userID, req.EndDate, req.Description,
	).Scan(&id); err != nil {
		slog.ErrorContext(ctx,
			"error inserting new offering on the database",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return nil, ErrServer
	}

	enrollmentCode := IDToCode(id)

	if _, err = tx.ExecContext(ctx,
		"UPDATE offerings SET enrollment_code = $1 WHERE id = $2",
		enrollmentCode, id,
	); err != nil {
		slog.ErrorContext(ctx,
			"error updating offering enrollment_code",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
			slog.Int64("offering_id", id),
			slog.String("enrollment_code", enrollmentCode),
		)
		return nil, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx,
			"error committing database transaction",
			slog.String("error", err.Error()),
			slog.Any("user_id", claims["id"]),
		)
		return nil, ErrServer
	}

	cache.Delete(ctx, cache.OfferingKey(id))

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

	if _, err := database.DB.ExecContext(ctx, `
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

	cache.Delete(ctx, cache.OfferingKey(offering.ID))

	slog.InfoContext(ctx, "offering updated",
		slog.Int64("offering_id", offering.ID),
	)

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
storageObject names one object to clean up, and which bucket holds it. The S3
object keys of the different kinds overlap in shape (a case id and an exercise
id are both numbers), so the bucket travels with the key instead of being
re-derived from it.
*/
type storageObject struct {
	inCases bool
	key     string
}

/*
offeringStorageKeys collects the objects an offering owns: test case inputs,
outputs and extra files, compilation files, attachments and submitted sources.
They are removed after the rows are gone, so a deleted class does not leave its
authoring content and submissions behind in the object store.
*/
func offeringStorageKeys(ctx context.Context, offeringID int64) ([]storageObject, error) {
	keys := make([]storageObject, 0, 16)

	rows, err := database.DB.QueryContext(ctx, `
		SELECT tc.id, tc.input_type::text, tc.expected_output_type::text,
		       COALESCE(files.path, '')
		FROM exercises_test_cases tc
		JOIN exercises e ON e.id = tc.exercise_id
		LEFT JOIN exercises_test_cases_files files
		       ON files.exercise_test_case_id = tc.id
		WHERE e.offering_id = $1`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting test case objects",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			caseID       int64
			inputType    string
			outputType   string
			attachedPath string
		)
		if err := rows.Scan(&caseID, &inputType, &outputType, &attachedPath); err != nil {
			slog.ErrorContext(ctx, "error scanning test case object",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		if inputType == IOTypeFile {
			keys = append(keys, storageObject{true, storage.CaseInputKey(caseID)})
		}
		if outputType == IOTypeFile {
			keys = append(keys, storageObject{true, storage.CaseOutputKey(caseID)})
		}
		if attachedPath != "" {
			keys = append(keys, storageObject{
				true, storage.CaseFileKey(caseID, attachedPath),
			})
		}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating test case objects",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err = database.DB.QueryContext(ctx, `
		SELECT files.exercise_id, files.path, 'compilation'
		FROM exercises_compilation_files files
		JOIN exercises e ON e.id = files.exercise_id
		WHERE e.offering_id = $1
		UNION ALL
		SELECT files.exercise_id, files.path, 'attachment'
		FROM exercises_attached_files files
		JOIN exercises e ON e.id = files.exercise_id
		WHERE e.offering_id = $1`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting exercise file objects",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var (
			exerciseID int64
			filePath   string
			kind       string
		)
		if err := rows.Scan(&exerciseID, &filePath, &kind); err != nil {
			slog.ErrorContext(ctx, "error scanning exercise file object",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		if kind == "compilation" {
			keys = append(keys, storageObject{
				false, storage.CompilationFileKey(exerciseID, filePath),
			})
		} else {
			keys = append(keys, storageObject{
				false, storage.AttachmentKey(exerciseID, filePath),
			})
		}
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating exercise file objects",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err = database.DB.QueryContext(ctx, `
		SELECT c.s3_key
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		WHERE e.offering_id = $1 AND c.s3_key IS NOT NULL`, offeringID,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error collecting submitted sources",
			slog.String("error", err.Error()),
			slog.Int64("offering_id", offeringID),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			slog.ErrorContext(ctx, "error scanning submitted source",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		keys = append(keys, storageObject{false, key})
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating submitted sources",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return keys, nil
}

/*
deleteObjectsBestEffort removes objects from the storage buckets, logging every
failure. A cleanup that cannot reach the object store must not fail the request:
the rows are already gone, so there is nothing left to retry against.
*/
func deleteObjectsBestEffort(ctx context.Context, objects []storageObject) {
	if _, _, err := storage.Client(); err != nil {
		slog.ErrorContext(ctx, "storage unavailable, skipping object cleanup",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, object := range objects {
		var err error
		if object.inCases {
			err = storage.DeleteCaseObject(ctx, object.key)
		} else {
			err = storage.DeleteFileObject(ctx, object.key)
		}
		if err != nil {
			slog.ErrorContext(ctx, "error deleting object",
				slog.String("key", object.key),
				slog.String("error", err.Error()),
			)
		}
	}
}

/*
ListOfferingMembers lists the people in a class: students who joined with the
code, monitors and co-professors.
*/
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

	email := strings.TrimSpace(req.Email)
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

const (
	alphabet string = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	base     int64  = int64(len(alphabet))      // 36
	space    int64  = base * base * base * base // 36^4 = 1,679,616
	prime    int64  = 1_276_043                 // Coprime with space (36^4 = 2^8 * 3^8, any prime ≠ 2,3 works)
)

/*
encode converts a non-negative integer "n" into a fixed-width 4-character
string using the constant "alphabet". It is the inverse of the base-36
positional notation, right-padded with the zero character ('A').

n must be in [0, space). Behaviour is undefined outside this range.
*/
func encode(n int64) string {
	digits := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		digits[i] = alphabet[n%base]
		n /= base
	}
	return string(digits)
}

/*
IDToCode converts a unique offering ID into a 4-character enrollment code.

The mapping is deterministic and collision-free: distinct IDs always produce
distinct codes. This is achieved through a multiplicative permutation —
multiplying by a prime coprime with the code space produces a bijection
over [0, space), so no uniqueness check against the database is needed.

The resulting codes appear non-sequential, making it harder for students to
guess or enumerate codes for other class offerings.

id must be in [0, 1,679,615]. If your dataset may exceed this range,
increase the code length or expand the alphabet before deploying.
*/
func IDToCode(id int64) string {
	scrambled := (id * prime) % space
	return encode(scrambled)
}
