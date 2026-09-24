package services

import (
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"golang.org/x/crypto/bcrypt"
)

const (
	// legacyPasswordPrefix marks password hashes carried over from the old
	// system by the database migration: 'legacy-sha1$' + hex(SHA-1(salt + plaintext)).
	legacyPasswordPrefix = "legacy-sha1$"

	// MaxUserNameLength and MaxOrgIDLength bound the profile fields.
	MaxUserNameLength = 100
	MaxOrgIDLength    = 64
)

/*
SignUp creates a new user on the database
*/
func SignUp(ctx context.Context, req *models.SignUpRequest) error {
	var password string
	var err error
	if password, err = hashPassword(req.Password); err != nil {
		slog.ErrorContext(ctx,
			"error hashing password",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	var tx *sql.Tx
	if tx, err = database.DB.BeginTx(ctx, nil); err != nil {
		slog.ErrorContext(ctx,
			"error initializing database transaction",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)",
		req.Name, req.Email, password,
	); err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code == pqerror.UniqueViolation {
				return ErrEmailExists
			}
		}
		slog.ErrorContext(ctx,
			"database error inserting new user",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx,
			"error committing database transaction",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	return nil
}

func LogIn(ctx context.Context, req *models.LogInRequest) (map[string]any, error) {
	var id int
	var name string
	var passwordHash string
	var role string
	if err := database.DB.QueryRowContext(ctx,
		"SELECT id, name, password_hash, role FROM users WHERE email = $1",
		req.Email).Scan(&id, &name, &passwordHash, &role); err != nil {
		if err == sql.ErrNoRows {
			slog.InfoContext(ctx,
				"someone tried to login as an user that does not exist",
			)
			return nil, ErrInvalidCredentials
		} else {
			slog.ErrorContext(ctx,
				"error querying database",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	}

	if strings.HasPrefix(passwordHash, legacyPasswordPrefix) {
		// Hash carried over from the old system: verify with the legacy
		// algorithm and upgrade to bcrypt on success (lazy migration).
		if err := verifyAndUpgradeLegacyPassword(
			ctx, id, req.Password, passwordHash,
		); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				slog.InfoContext(ctx,
					"provided password doesn't match with database",
				)
				return nil, ErrInvalidCredentials
			}
			slog.ErrorContext(ctx,
				"error verifying legacy password",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	} else {
		if err := verifyBcryptPassword(ctx, req.Password, passwordHash); err != nil {
			return nil, err
		}
	}

	claims := map[string]any{
		"id":    id,
		"name":  name,
		"email": req.Email,
		"role":  role,
	}

	jwtauth.SetIssuedAt(claims, time.Now())
	jwtauth.SetExpiryIn(claims, validation.SessionTTL)

	return claims, nil
}

/*
GetUserByID fetches a user by id.
*/
func GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User
	err := database.DB.QueryRowContext(ctx,
		"SELECT id, name, email, role FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		slog.ErrorContext(ctx,
			"error fetching user by id",
			slog.String("error", err.Error()),
			slog.Int("user_id", id),
		)
		return nil, ErrServer
	}

	return &user, nil
}

/*
CheckEmailExistence checks if the given email is already in use
*/
func CheckEmailExistence(ctx context.Context, email string) error {
	var id int
	err := database.DB.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1`,
		email,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		slog.ErrorContext(ctx,
			"database error validating email",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	return ErrEmailExists
}

/*
GetProfile returns the caller's own account.
*/
func GetProfile(ctx context.Context, userID int) (*models.Profile, error) {
	var profile models.Profile
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, email, COALESCE(org_id, ''), role, confirmed, created_at
		FROM users
		WHERE id = $1`, userID,
	).Scan(
		&profile.ID, &profile.Name, &profile.Email, &profile.OrgID,
		&profile.Role, &profile.Confirmed, &profile.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error fetching profile",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return &profile, nil
}

/*
UpdateProfile applies a partial update to the caller's own account. The role is
deliberately not updatable here: only an admin can grant one.
*/
func UpdateProfile(
	ctx context.Context, userID int, req *models.UpdateProfileRequest,
) (*models.Profile, error) {
	profile, err := GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		profile.Name = strings.TrimSpace(*req.Name)
		if err := validation.ValidateRequiredString(
			profile.Name, MaxUserNameLength,
		); err != nil {
			return nil, err
		}
	}
	if req.Email != nil {
		profile.Email = strings.TrimSpace(*req.Email)
		if err := validation.ValidateEmailFormat(ctx, profile.Email); err != nil {
			return nil, err
		}
	}
	if req.OrgID != nil {
		// Cleared on purpose: org_id is nullable, so an empty value removes it.
		profile.OrgID = strings.TrimSpace(*req.OrgID)
		if err := validation.ValidateOptionalString(
			profile.OrgID, MaxOrgIDLength,
		); err != nil {
			return nil, err
		}
	}

	if _, err := database.DB.ExecContext(ctx, `
		UPDATE users
		SET name = $1, email = $2, org_id = NULLIF($3, ''), updated_at = now()
		WHERE id = $4`,
		profile.Name, profile.Email, profile.OrgID, userID,
	); err != nil {
		if pgErr, ok := err.(*pq.Error); ok &&
			pgErr.Code == pqerror.UniqueViolation {
			switch pgErr.Constraint {
			case "users_email_key":
				return nil, ErrEmailExists
			case "users_org_id_key":
				return nil, ErrOrgIDExists
			}
		}
		slog.ErrorContext(ctx, "error updating profile",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "profile updated", slog.Int("user_id", userID))

	return profile, nil
}

/*
ChangePassword replaces the caller's password after checking the current one.

The session cookie is a stateless JWT with no server-side revocation list, so
tokens issued before the change stay valid until they expire; the browser's
session ends only because the client re-authenticates with the new password.
*/
func ChangePassword(
	ctx context.Context, userID int, req *models.ChangePasswordRequest,
) error {
	var stored string
	err := database.DB.QueryRowContext(ctx,
		"SELECT password_hash FROM users WHERE id = $1", userID,
	).Scan(&stored)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error fetching password hash",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if strings.HasPrefix(stored, legacyPasswordPrefix) {
		if err := verifyAndUpgradeLegacyPassword(
			ctx, userID, req.CurrentPassword, stored,
		); err != nil {
			return err
		}
	} else if err := verifyBcryptPassword(
		ctx, req.CurrentPassword, stored,
	); err != nil {
		return err
	}

	hashed, err := hashPassword(req.NewPassword)
	if err != nil {
		slog.ErrorContext(ctx, "error hashing password",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if _, err := database.DB.ExecContext(ctx,
		"UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2",
		hashed, userID,
	); err != nil {
		slog.ErrorContext(ctx, "error updating password",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	slog.InfoContext(ctx, "password changed", slog.Int("user_id", userID))

	return nil
}

/*
verifyAndUpgradeLegacyPassword checks a password against a legacy hash
('legacy-sha1$' + hex(SHA-1(salt + plaintext))) brought over by the database
migration and, on success, replaces it with a bcrypt hash so the legacy format
disappears after the first login.
*/
func verifyAndUpgradeLegacyPassword(
	ctx context.Context, userID int, password, stored string,
) error {
	salt := config.Get().LegacyPasswordSalt
	if salt == "" {
		slog.ErrorContext(ctx,
			"legacy password hash found but RUNCODES_LEGACY_PASSWORD_SALT is not set",
			slog.Int("user_id", userID),
		)
		return ErrServer
	}

	// SHA-1 is used here only to verify hashes created by the legacy system;
	// it is never used to store new passwords.
	sum := sha1.Sum([]byte(salt + password))
	legacyHex := strings.TrimPrefix(stored, legacyPasswordPrefix)
	if subtle.ConstantTimeCompare(
		[]byte(legacyHex), []byte(hex.EncodeToString(sum[:])),
	) != 1 {
		return ErrInvalidCredentials
	}

	upgraded, err := hashPassword(password)
	if err != nil {
		slog.ErrorContext(ctx,
			"error hashing password",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if _, err := database.DB.ExecContext(ctx,
		"UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2",
		upgraded, userID,
	); err != nil {
		// Non-fatal: the login still proceeds and the upgrade is retried on
		// the next login.
		slog.ErrorContext(ctx,
			"error upgrading legacy password hash",
			slog.String("error", err.Error()),
			slog.Int("user_id", userID),
		)
	}

	return nil
}

/*
verifyBcryptPassword checks a password against a stored bcrypt hash.

A stored value the library cannot parse at all — the locked account the seed
creates, or a corrupted row — matches no password, and is answered exactly like
a wrong password: telling the two apart would report the state of an account to
whoever asks for it. The reason is kept in the log for the operator instead.
*/
func verifyBcryptPassword(ctx context.Context, password, stored string) error {
	err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(password))
	if err == nil {
		return nil
	}

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		slog.InfoContext(ctx,
			"provided password doesn't match with database",
		)
	} else {
		slog.WarnContext(ctx,
			"stored password hash cannot be verified",
			slog.String("error", err.Error()),
		)
	}

	return ErrInvalidCredentials
}

/*
hashPassword takes a password and returns a hashed password
*/
func hashPassword(password string) (string, error) {
	if bytes, err := bcrypt.GenerateFromPassword(
		[]byte(password), 12,
	); err != nil {
		return "", err
	} else {
		return string(bytes), nil
	}
}
