// Package utils provides utility functions and configurations for the application.
package utils

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func configureEnv() map[string]string {
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
			Logger.Error(fmt.Sprintf("%s environment variable is not set", field))
			return nil
		}
		env[field] = value
	}

	return env
}

func InitDB() error {
	env := configureEnv()

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env["RUNCODES_DB_HOST"], env["RUNCODES_DB_PORT"], env["RUNCODES_DB_USER"], env["RUNCODES_DB_PASSWORD"], env["RUNCODES_DB_NAME"],
	)

	var err error

	DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		Logger.Error("Failed to open database connection", slog.String("error", err.Error()))
		return err
	}

	if err = DB.Ping(); err != nil {
		Logger.Error("Failed to ping database", slog.String("error", err.Error()))
		return err
	}

	Logger.Info("Connected to the database successfully")

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)
	return nil
}
