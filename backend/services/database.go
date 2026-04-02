package services

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func configureEnv() (map[string]string, error) {
	envFields := []string{
		"RUNCODES_DB_HOST",
		"RUNCODES_DB_PORT",
		"RUNCODES_DB_USER",
		"RUNCODES_DB_PASSWORD",
		"RUNCODES_DB_NAME",
	}

	env := make(map[string]string)

	for _, field := range envFields {
		value := os.Getenv(field)
		if value == "" {
			err := fmt.Sprintf("%s environment variable is not set", field)
			slog.Error(err)
			return nil, errors.New(err)
		}
		env[field] = value
	}

	return env, nil
}

/*
InitDB configures and connects to the database using values from the .env file or environment variables.

The database can be accessed through the utils.DB variable.
*/
func InitDB() error {
	var env map[string]string
	var err error
	if env, err = configureEnv(); err != nil {
		slog.Error("Failed to retrieve environment variables for database connection")
		return err
	}

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env["RUNCODES_DB_HOST"], env["RUNCODES_DB_PORT"], env["RUNCODES_DB_USER"], env["RUNCODES_DB_PASSWORD"], env["RUNCODES_DB_NAME"],
	)

	if DB, err = sql.Open("postgres", connectionString); err != nil {
		slog.Error("Failed to open database connection", slog.String("error", err.Error()))
		return err
	}

	if err := DB.Ping(); err != nil {
		slog.Error("Failed to ping database", slog.String("error", err.Error()))
		return err
	}

	slog.Info("Connected to the database successfully")

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)
	return nil
}
