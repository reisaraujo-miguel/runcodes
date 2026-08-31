package services

import (
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"runcodes/models"
	"runcodes/validation"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"golang.org/x/crypto/bcrypt"
)

const (
	// legacyPasswordPrefix marks password hashes carried over from the old
	// system by the database migration: 'legacy-sha1$' + hex(SHA-1(salt + plaintext)).
	legacyPasswordPrefix  = "legacy-sha1$"
	legacyPasswordSaltEnv = "RUNCODES_LEGACY_PASSWORD_SALT"
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
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
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
	if err := DB.QueryRowContext(ctx,
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
		if err := bcrypt.CompareHashAndPassword(
			[]byte(passwordHash), []byte(req.Password),
		); err != nil {
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				slog.InfoContext(ctx,
					"provided password doesn't match with database",
					slog.String("error", err.Error()),
				)
				return nil, ErrInvalidCredentials
			}
			slog.ErrorContext(ctx,
				"error comparing hash and password",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
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
	err := DB.QueryRowContext(ctx,
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
	err := DB.QueryRowContext(ctx,
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
verifyAndUpgradeLegacyPassword checks a password against a legacy hash
('legacy-sha1$' + hex(SHA-1(salt + plaintext))) brought over by the database
migration and, on success, replaces it with a bcrypt hash so the legacy format
disappears after the first login.
*/
func verifyAndUpgradeLegacyPassword(
	ctx context.Context, userID int, password, stored string,
) error {
	salt := os.Getenv(legacyPasswordSaltEnv)
	if salt == "" {
		slog.ErrorContext(ctx,
			"legacy password hash found but the salt environment variable is not set",
			slog.String("env_var", legacyPasswordSaltEnv),
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

	if _, err := DB.ExecContext(ctx,
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
