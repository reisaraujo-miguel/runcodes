// Package utils provides utility functions and configurations for the application.
package utils

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

func configureEnv() (error, map[string]string) {
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
			Logger.Error(err)
			return errors.New(err), nil
		}
		env[field] = value
	}

	return nil, env
}

// Configures and connects to the database using the values defined on the .env file or environment variables.
//
// The database can be accessed through the utils.DB variable.
func InitDB() error {
	err, env := configureEnv()
	if err != nil {
		Logger.Error("Failed to retrieve environment variables for database connection")
		return err
	}

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env["RUNCODES_DB_HOST"], env["RUNCODES_DB_PORT"], env["RUNCODES_DB_USER"], env["RUNCODES_DB_PASSWORD"], env["RUNCODES_DB_NAME"],
	)

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
