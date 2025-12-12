package utils

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"runcodes/errors"

	_ "github.com/lib/pq"
)

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
			errors.LogFatalError("databse.go", "configureEnv", fmt.Sprintf("%s environment variable is not set", field), nil)
		}
		env[field] = value
	}

	return env
}

func InitDB() *sql.DB {
	env := configureEnv()

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env["RUNCODES_DB_HOST"], env["RUNCODES_DB_PORT"], env["RUNCODES_DB_USER"], env["RUNCODES_DB_PASSWORD"], env["RUNCODES_DB_NAME"],
	)

	var err error
	var db *sql.DB

	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		errors.LogFatalError("database.go", "InitDB", "Failed to open database connection", err)
	}

	if err = db.Ping(); err != nil {
		errors.LogFatalError("database.go", "InitDB", "Failed to ping database", err)
	}

	log.Println("Connected to the database successfully")

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db
}
