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
		msg := "database error creating offering"
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
hashPassword takes a password and returns a hashed password
*/
func hashPassword(password string) (string, error) {
	if bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12); err != nil {
		return "", err
	} else {
		return string(bytes), nil
	}
}
