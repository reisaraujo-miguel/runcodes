package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/storage"
)

/*
The files a submission generates are uploaded to the outputfiles bucket while the
run advances. Nothing in the database references them, so without this sweep they
would stay forever: every run that writes a file would add objects no one can
find from a class or a submission once it is old.

The sweep runs in the API process, like the other notifiers. It is disabled when
the retention is 0, which leaves the objects to the bucket's own lifecycle.
*/

// outputRetentionInterval is how often the sweep looks for expired files. An hour
// is far below the default week-long retention and cheap enough to ignore.
const outputRetentionInterval = time.Hour

/* StartOutputFileRetention deletes generated files older than the configured TTL. */
func StartOutputFileRetention(ctx context.Context) {
	ttl := config.Get().OutputFiles.TTL
	if ttl <= 0 {
		slog.Info("generated file retention is disabled; objects are kept")
		return
	}

	go func() {
		slog.Info("generated file retention started", slog.Duration("ttl", ttl))

		// Sweep once at startup: an instance restarted more often than the
		// interval would otherwise never delete anything.
		purgeExpiredOutputFiles(ctx, ttl)

		ticker := time.NewTicker(outputRetentionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				purgeExpiredOutputFiles(ctx, ttl)
			}
		}
	}()
}

/*
purgeExpiredOutputFiles removes the generated files whose upload is older than the
retention. A storage failure is logged and retried on the next tick: a sweep that
missed one round only delays the deletion.
*/
func purgeExpiredOutputFiles(ctx context.Context, ttl time.Duration) {
	removed, err := storage.DeleteExpiredOutputFiles(ctx, time.Now().Add(-ttl))
	if err != nil {
		slog.ErrorContext(ctx, "error deleting expired generated files",
			slog.String("error", err.Error()),
		)
		return
	}

	if removed > 0 {
		slog.InfoContext(ctx, "deleted expired generated files",
			slog.Int("files", removed),
			slog.Duration("ttl", ttl),
		)
	}
}
