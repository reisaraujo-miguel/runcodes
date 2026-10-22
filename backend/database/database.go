/*
Package database owns the process-wide PostgreSQL connection pool.
*/
package database

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/config"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
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

The driver is wrapped with otelsql so every query that receives a context
produces a database span and the pool exports connection metrics. If the wrapper
cannot be registered the plain lib/pq driver is used: instrumentation must never
prevent the API from starting.
*/
func InitDB() error {
	cfg := config.Get().DB

	driverName := "postgres"
	instrumented, err := otelsql.Register("postgres",
		otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
	)
	if err != nil {
		slog.Warn("could not instrument the database driver; using lib/pq directly",
			slog.String("error", err.Error()),
		)
	} else {
		driverName = instrumented
	}

	if DB, err = sql.Open(driverName, cfg.DSN()); err != nil {
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

	// Connection-pool metrics (in use, idle, wait counts) are exported through
	// the global meter provider configured by the telemetry package.
	if _, err := otelsql.RegisterDBStatsMetrics(DB,
		otelsql.WithAttributes(semconv.DBSystemNamePostgreSQL),
	); err != nil {
		slog.Debug("could not register database pool metrics",
			slog.String("error", err.Error()),
		)
	}

	slog.Info("Connected to the database successfully")

	DB.SetMaxOpenConns(maxOpenConns)
	DB.SetMaxIdleConns(maxIdleConns)
	DB.SetConnMaxLifetime(connMaxLifetime)
	return nil
}
