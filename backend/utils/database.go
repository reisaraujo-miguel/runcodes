// Package utils provides utility functions and configurations for the application.
package utils

import (
	"database/sql"
	"fmt"
	"log"
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
			log.Fatalf("%s environment variable is not set", field)
		}
		env[field] = value
	}

	return env
}

func InitDB() {
	env := configureEnv()

	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env["RUNCODES_DB_HOST"], env["RUNCODES_DB_PORT"], env["RUNCODES_DB_USER"], env["RUNCODES_DB_PASSWORD"], env["RUNCODES_DB_NAME"],
	)

	var err error

	DB, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatalf("Failed to open database connection: %s\n", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %s\n", err)
	}

	log.Println("Connected to the database successfully")

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)
}
