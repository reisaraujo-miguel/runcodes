package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
)

/*
Readiness is the readiness probe used by orchestrators and the blackbox
exporter. It checks the database, which every request needs, and reports the
judge separately: a judge outage breaks submissions but not authentication,
courses or browsing, so it must not remove the API from a load balancer.
*/
func Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	body := map[string]string{
		"database": "ok",
		"judge":    "ok",
		"status":   "ok",
	}
	status := http.StatusOK

	if err := databasePing(ctx); err != nil {
		slog.WarnContext(ctx, "readiness: database unavailable",
			slog.String("error", err.Error()),
		)
		body["database"] = "unavailable"
		body["status"] = "unavailable"
		status = http.StatusServiceUnavailable
	}

	// The judge is informational: do not fail readiness, and do not log a
	// warning every probe while it is down.
	if err := judgePing(ctx); err != nil {
		slog.DebugContext(ctx, "readiness: judge unavailable",
			slog.String("error", err.Error()),
		)
		body["judge"] = "unavailable"
	}

	WriteResponse(w, status, body)
}

// The indirections keep Ready testable without a real PostgreSQL or judge.
var (
	databasePing = func(ctx context.Context) error {
		if database.DB == nil {
			return errors.New("database is not initialized")
		}
		return database.DB.PingContext(ctx)
	}
	judgePing = judge.Ready
)
