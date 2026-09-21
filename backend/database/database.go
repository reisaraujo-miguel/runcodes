/*
Package database owns the process-wide PostgreSQL connection pool.
*/
package database

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/config"

	_ "github.com/lib/pq"
)

// Pool sizing. The pool is shared by every request handler and by the judge
// background services, so it is sized for the API's concurrency rather than for
// a single caller.
const (
	maxOpenConns    = 25
	maxIdleConns    = 25
	connMaxLifetime = 5 * time.Minute
)

// DB is the connection pool used by the whole service.
var DB *sql.DB

/*
InitDB connects to the database described by the configuration and exposes the
pool as DB.
*/
func InitDB() error {
	cfg := config.Get().DB

	var err error
	if DB, err = sql.Open("postgres", cfg.DSN()); err != nil {
		slog.Error(
			"Failed to open database connection",
			slog.String("error", err.Error()),
		)
		return err
	}

	if err := DB.Ping(); err != nil {
		slog.Error("Failed to ping database", slog.String("error", err.Error()))
		return err
	}

	slog.Info("Connected to the database successfully")

	DB.SetMaxOpenConns(maxOpenConns)
	DB.SetMaxIdleConns(maxIdleConns)
	DB.SetConnMaxLifetime(connMaxLifetime)
	return nil
}
