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
than the timeout as server_error, so a crashed judge cannot leave commits stuck
forever, and releases the event hub of each one: the run can no longer report
anything, so keeping its judge stream open would leak a goroutine and a
connection per crashed run.
*/
func reconcileStaleCommits(ctx context.Context, timeout time.Duration) {
	rows, err := database.DB.QueryContext(ctx,
		`UPDATE commits
		 SET status = 'server_error',
		     compilation_finished = COALESCE(compilation_finished, now())
		 WHERE status IN ('compiling', 'running')
		   AND compilation_started IS NOT NULL
		   AND compilation_started < now() - make_interval(secs => $1)
		 RETURNING id`,
		timeout.Seconds(),
	)
	if err != nil {
		slog.ErrorContext(ctx, "error reconciling stale commits",
			slog.String("error", err.Error()),
		)
		return
	}
	defer rows.Close()

	var commitIDs []int64
	for rows.Next() {
		var commitID int64
		if err := rows.Scan(&commitID); err != nil {
			slog.ErrorContext(ctx, "error scanning a reconciled commit",
				slog.String("error", err.Error()),
			)
			return
		}
		commitIDs = append(commitIDs, commitID)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating reconciled commits",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, commitID := range commitIDs {
		CloseHub(commitID)
	}

	if len(commitIDs) > 0 {
		slog.WarnContext(ctx, "marked stale commits as server_error",
			slog.Int("commits", len(commitIDs)),
			slog.Duration("stale_timeout", timeout),
		)
	}
}
