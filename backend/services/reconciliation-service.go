package services

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const (
	staleTimeoutEnv = "RUNCODES_JUDGE_STALE_TIMEOUT"

	defaultStaleTimeout = 15 * time.Minute
	reconcileInterval   = 30 * time.Second
)

/*
StartReconciliation runs the judge reconciliation sweeper until ctx is
cancelled. Every 30s it marks commits stuck in compiling/running (claimed but
never finished) as server_error.
*/
func StartReconciliation(ctx context.Context) {
	timeout := staleTimeout()

	slog.InfoContext(ctx, "judge reconciliation sweeper started",
		slog.Duration("stale_timeout", timeout),
		slog.Duration("interval", reconcileInterval),
	)

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcileStaleCommits(ctx, timeout)
		}
	}
}

/*
staleTimeout reads RUNCODES_JUDGE_STALE_TIMEOUT, falling back to the default
when unset or invalid.
*/
func staleTimeout() time.Duration {
	raw := os.Getenv(staleTimeoutEnv)
	if raw == "" {
		return defaultStaleTimeout
	}

	timeout, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid judge stale timeout, using default",
			slog.String("value", raw),
			slog.String("error", err.Error()),
			slog.Duration("default", defaultStaleTimeout),
		)
		return defaultStaleTimeout
	}

	if timeout <= 0 {
		slog.Warn("judge stale timeout must be positive, using default",
			slog.String("value", raw),
			slog.Duration("default", defaultStaleTimeout),
		)
		return defaultStaleTimeout
	}

	return timeout
}

/*
reconcileStaleCommits marks claimed commits whose compilation_started is older
than the timeout as server_error, so a crashed judge cannot leave commits
stuck forever.
*/
func reconcileStaleCommits(ctx context.Context, timeout time.Duration) {
	result, err := DB.ExecContext(ctx,
		`UPDATE commits
		 SET status = 'server_error',
		     compilation_finished = COALESCE(compilation_finished, now())
		 WHERE status IN ('compiling', 'running')
		   AND compilation_started IS NOT NULL
		   AND compilation_started < now() - make_interval(secs => $1)`,
		timeout.Seconds(),
	)
	if err != nil {
		slog.ErrorContext(ctx, "error reconciling stale commits",
			slog.String("error", err.Error()),
		)
		return
	}

	if affected, err := result.RowsAffected(); err == nil && affected > 0 {
		slog.WarnContext(ctx, "marked stale commits as server_error",
			slog.Int64("commits", affected),
			slog.Duration("stale_timeout", timeout),
		)
	}
}
