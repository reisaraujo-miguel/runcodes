package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

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
		msg := "error hashing password"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	var tx *sql.Tx
	if tx, err = DB.BeginTx(ctx, nil); err != nil {
		msg := "error initializing the transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)",
		req.UserName, req.Email, password,
	); err != nil {
		msg := "database error registering user"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	if err := tx.Commit(); err != nil {
		msg := "error during database transaction"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return errors.New(msg)
	}

	return nil
}

/*
CheckEmailExistence returns if there is an user registered with the given email
*/
func CheckEmailExistence(ctx context.Context, email string) (bool, error) {
	var id int
	err := DB.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = $1`,
		email,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		msg := "database error validating email"
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		return false, errors.New(msg)
	}

	return true, nil // email exists
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
