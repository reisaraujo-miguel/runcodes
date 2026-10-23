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

// dummyPasswordHash is a valid bcrypt hash (cost 12) checked when a login names
// an unknown account. Without it the missing-user path returns before bcrypt and
// is hundreds of milliseconds faster than a wrong password, which reveals
// whether an address is registered.
const dummyPasswordHash = "$2a$12$OBQ6gUTivqqVNZ9Kp0ig3eajxQGbpRMB5wXWMnTE1Jnjd40frbetq"

/*
SignUp creates a new user on the database and returns its id. The account is
created unconfirmed: the caller sends the verification email, and LogIn refuses
it until the link is used.
*/
func SignUp(ctx context.Context, req *models.SignUpRequest) (int, error) {
	req.Email = validation.NormalizeEmail(req.Email)

	var password string
	var err error
	if password, err = hashPassword(req.Password); err != nil {
		slog.ErrorContext(ctx,
			"error hashing password",
			slog.String("error", err.Error()),
		)
		return 0, ErrServer
	}

	var tx *sql.Tx
	if tx, err = database.DB.BeginTx(ctx, nil); err != nil {
		slog.ErrorContext(ctx,
			"error initializing database transaction",
			slog.String("error", err.Error()),
		)
		return 0, ErrServer
	}

	defer tx.Rollback()

	var userID int
	if err = tx.QueryRowContext(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Name, req.Email, password,
	).Scan(&userID); err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code == pqerror.UniqueViolation {
				return 0, ErrEmailExists
			}
		}
		slog.ErrorContext(ctx,
			"database error inserting new user",
			slog.String("error", err.Error()),
		)
		return 0, ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx,
			"error committing database transaction",
			slog.String("error", err.Error()),
		)
		return 0, ErrServer
	}

	return userID, nil
}

func LogIn(ctx context.Context, req *models.LogInRequest) (map[string]any, error) {
	email := validation.NormalizeEmail(req.Email)

	var id int
	var name string
	var passwordHash string
	var role string
	var confirmed bool
	var tokenVersion int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT id, name, password_hash, role, confirmed, token_version FROM users WHERE email = $1",
		email).Scan(&id, &name, &passwordHash, &role, &confirmed, &tokenVersion); err != nil {
		if err == sql.ErrNoRows {
			// Spend the same bcrypt time as a wrong password on a real account;
			// the result is intentionally ignored.
			_ = bcrypt.CompareHashAndPassword(
				[]byte(dummyPasswordHash), []byte(req.Password),
			)
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

	// Checked after the password so an unverified account is only revealed to
	// someone who already knows its credentials.
	if !confirmed {
		slog.InfoContext(ctx,
			"someone tried to login before verifying their email",
			slog.Int("user_id", id),
		)
		return nil, ErrEmailNotVerified
	}

	claims := map[string]any{
		"id":            id,
		"name":          name,
		"email":         email,
		"role":          role,
		"token_version": tokenVersion,
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
		"SELECT id, name, email, role, token_version FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.TokenVersion)
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
	email = validation.NormalizeEmail(email)

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
		SELECT id, name, email, COALESCE(org_id, ''), role, confirmed,
		       code_editor_enabled, created_at, COALESCE(pending_email, '')
		FROM users
		WHERE id = $1`, userID,
	).Scan(
		&profile.ID, &profile.Name, &profile.Email, &profile.OrgID,
		&profile.Role, &profile.Confirmed, &profile.CodeEditorEnabled,
		&profile.CreatedAt, &profile.PendingEmail,
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

An email change is a two-step flow: the caller proves knowledge of the current
password, the new address is stored as pending_email and receives a verification
link, and the advertised email only changes when that link is used. The current
address stays usable in the meantime, so a typo or an undeliverable address
cannot lock the account out.
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
	if req.OrgID != nil {
		// Cleared on purpose: org_id is nullable, so an empty value removes it.
		profile.OrgID = strings.TrimSpace(*req.OrgID)
		if err := validation.ValidateOptionalString(
			profile.OrgID, MaxOrgIDLength,
		); err != nil {
			return nil, err
		}
	}
	if req.CodeEditorEnabled != nil {
		profile.CodeEditorEnabled = *req.CodeEditorEnabled
	}

	emailChanged := false
	if req.Email != nil {
		newEmail := validation.NormalizeEmail(*req.Email)
		if newEmail != validation.NormalizeEmail(profile.Email) {
			if req.CurrentPassword == "" {
				return nil, ErrCurrentPasswordRequired
			}
			if err := verifyCurrentPassword(
				ctx, userID, req.CurrentPassword,
			); err != nil {
				return nil, err
			}
			if err := validation.ValidateEmailFormat(ctx, newEmail); err != nil {
				return nil, err
			}
			if err := checkEmailAvailable(ctx, userID, newEmail); err != nil {
				return nil, err
			}

			// Not applied yet: the current address remains active until the link
			// sent to newEmail is consumed.
			profile.PendingEmail = newEmail
			emailChanged = true
		}
	}

	if _, err := database.DB.ExecContext(ctx, `
		UPDATE users
		SET name = $1, email = $2, org_id = NULLIF($3, ''), updated_at = now(),
		    code_editor_enabled = $4,
		    pending_email = CASE WHEN $5 = '' THEN pending_email ELSE $5 END
		WHERE id = $6`,
		profile.Name, profile.Email, profile.OrgID,
		profile.CodeEditorEnabled, profile.PendingEmail, userID,
	); err != nil {
		if pgErr, ok := err.(*pq.Error); ok &&
			pgErr.Code == pqerror.UniqueViolation {
			switch pgErr.Constraint {
			case "users_email_key", "users_pending_email_key",
				"users_email_lower_key", "users_pending_email_lower_key":
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

	if emailChanged {
		if err := SendEmailChangeEmail(
			ctx, userID, profile.Name, profile.PendingEmail,
		); err != nil {
			// Do not leave a pending address no link can confirm. The other
			// profile fields are already applied and remain valid.
			if _, clearErr := database.DB.ExecContext(ctx,
				`UPDATE users SET pending_email = NULL
				 WHERE id = $1 AND pending_email = $2`,
				userID, profile.PendingEmail,
			); clearErr != nil {
				slog.ErrorContext(ctx, "error clearing pending email after send failure",
					slog.Int("user_id", userID),
					slog.String("error", clearErr.Error()),
				)
			}
			slog.ErrorContext(ctx, "error sending email change verification",
				slog.Int("user_id", userID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	}

	slog.InfoContext(ctx, "profile updated",
		slog.Int("user_id", userID),
		slog.Bool("email_change_pending", emailChanged),
	)

	return profile, nil
}

/*
checkEmailAvailable rejects an address already used by another account, either as
its current email or as an in-flight email change.
*/
func checkEmailAvailable(ctx context.Context, userID int, email string) error {
	var id int
	err := database.DB.QueryRowContext(ctx, `
		SELECT id FROM users
		WHERE id <> $1 AND (email = $2 OR pending_email = $2)`,
		userID, email,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		slog.ErrorContext(ctx, "error checking email availability",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	return ErrEmailExists
}

/*
verifyCurrentPassword checks a password against the stored hash, upgrading a
legacy hash on success. It is the shared step behind password changes and email
changes.
*/
func verifyCurrentPassword(ctx context.Context, userID int, password string) error {
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
		return verifyAndUpgradeLegacyPassword(ctx, userID, password, stored)
	}
	return verifyBcryptPassword(ctx, password, stored)
}

/*
ChangePassword replaces the caller's password after checking the current one.
The password update bumps token_version, so every session issued before the
change is rejected on its next request instead of surviving until it expires.
*/
func ChangePassword(
	ctx context.Context, userID int, req *models.ChangePasswordRequest,
) error {
	if err := verifyCurrentPassword(ctx, userID, req.CurrentPassword); err != nil {
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
		`UPDATE users
		 SET password_hash = $1, token_version = token_version + 1, updated_at = NOW()
		 WHERE id = $2`,
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
