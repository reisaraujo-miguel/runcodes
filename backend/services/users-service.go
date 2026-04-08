package services

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"runcodes/models"

	"golang.org/x/crypto/bcrypt"
)

/*
SignUp creates a new user on the database
*/
func SignUp(ctx context.Context, req *models.SignUpRequest) error {
	var password string
	var err error
	if password, err = hashPassword(req.Password); err != nil {
		slog.ErrorContext(ctx, "error hashing password", slog.String("error", err.Error()))
		return ErrServer
	}

	var tx *sql.Tx
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
		slog.ErrorContext(ctx, "error initializing database transaction", slog.String("error", err.Error()))
		return ErrServer
	}

	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)",
		req.Name, req.Email, password,
	); err != nil {
		slog.ErrorContext(ctx, "database error inserting new user", slog.String("error", err.Error()))
		return ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing database transaction", slog.String("error", err.Error()))
		return ErrServer
	}

	return nil
}

func LogIn(ctx context.Context, req *models.LogInRequest) (map[string]any, error) {
	var id int
	var name string
	var passwordHash string
	if err := DB.QueryRowContext(ctx,
		"SELECT id, name, password_hash FROM users WHERE email = $1",
		req.Email).Scan(&id, &name, &passwordHash); err != nil {
		if err == sql.ErrNoRows {
			slog.InfoContext(ctx, "someone tried to login as an user that does not exist")
			return nil, ErrUserNotFound
		} else {
			slog.ErrorContext(ctx, "error querying database", slog.String("error", err.Error()))
			return nil, ErrServer
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		slog.InfoContext(ctx, "provided password doesn't match with database", slog.String("error", err.Error()))
		return nil, ErrInvalidPassword
	}

	claims := map[string]any{
		"id":    id,
		"name":  name,
		"email": req.Email,
		"exp":   time.Now().Add(30 * time.Minute),
		"iat":   time.Now(),
	}

	return claims, nil
}

/*
CheckEmailExistence returns if there is an user registered with the given email
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
		slog.ErrorContext(ctx, "database error validating email", slog.String("error", err.Error()))
		return ErrServer
	}

	return ErrEmailExists
}

/*
hashPassword takes a password and returns a hashed password
*/
func hashPassword(password string) (string, error) {
	if bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12); err != nil {
		return "", err
	} else {
		return string(bytes), nil
	}
}
