package database

import (
	"context"
	"database/sql/driver"
	"hash/fnv"
	"log/slog"
	"time"
)

/*
RunWithBackgroundLock runs fn only when this process can take the named
Postgres advisory lock, so a periodic job (an alert aggregation, a retention
sweep) runs in one backend instance at a time. The lock is session-scoped and
held on a dedicated connection for the duration of fn; losing it on shutdown is
harmless because the next tick of any instance will run the job again.

When the database is not configured (unit tests) fn runs directly, which keeps
callers simple.
*/
func RunWithBackgroundLock(ctx context.Context, name string, fn func()) {
	if DB == nil {
		fn()
		return
	}

	conn, err := DB.Conn(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not reserve a connection for a background job",
			slog.String("job", name),
			slog.String("error", err.Error()),
		)
		return
	}

	key := advisoryLockKey(name)
	var acquired bool
	if err := conn.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock($1)`, key,
	).Scan(&acquired); err != nil {
		_ = conn.Close()
		slog.ErrorContext(ctx, "could not acquire the background job lock",
			slog.String("job", name),
			slog.String("error", err.Error()),
		)
		return
	}
	if !acquired {
		_ = conn.Close()
		slog.DebugContext(ctx, "background job is running in another instance",
			slog.String("job", name),
		)
		return
	}

	defer func() {
		// The application context may already be cancelled during shutdown; the
		// unlock and the connection hand-back still have to happen.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := conn.ExecContext(unlockCtx,
			`SELECT pg_advisory_unlock($1)`, key,
		); err != nil {
			// The session may still own the lock. Do not hand the connection back
			// to the pool, where the stale lock would block every later pass.
			_ = conn.Raw(func(driverConn any) error {
				return driver.ErrBadConn
			})
			slog.Warn("could not release the background job lock; discarding the connection",
				slog.String("job", name),
				slog.String("error", err.Error()),
			)
		}
		_ = conn.Close()
	}()

	fn()
}

// advisoryLockKey derives a stable 64-bit key from the job name. The database
// only sees the number, so the name is documentation for the reader.
func advisoryLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))

	return int64(h.Sum64())
}
