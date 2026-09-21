package judge

import (
	"context"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
)

// reconcileInterval is how often the sweeper looks for stale commits.
const reconcileInterval = 30 * time.Second

/*
StartReconciliation runs the judge reconciliation sweeper until ctx is
cancelled. Every 30s it marks commits stuck in compiling/running (claimed but
never finished) for longer than the configured stale timeout as server_error.
*/
func StartReconciliation(ctx context.Context) {
	timeout := config.Get().Judge.StaleTimeout

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
reconcileStaleCommits marks claimed commits whose compilation_started is older
than the timeout as server_error, so a crashed judge cannot leave commits
stuck forever.
*/
func reconcileStaleCommits(ctx context.Context, timeout time.Duration) {
	result, err := database.DB.ExecContext(ctx,
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
