package engine

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
)

// sweepInterval is how often the judge looks for orphaned workspaces. Hourly is
// cheap enough to ignore and frequent enough to reclaim a crashed run's disk.
const sweepInterval = time.Hour

// workspaceGrace is added to the maximum run duration before a workspace is
// considered abandoned: a run that is still active must never be swept.
const workspaceGrace = 5 * time.Minute

/*
StartSweeper reclaims abandoned workspaces, then repeats hourly until ctx is
cancelled. The initial sweep also covers whatever a previous judge process left
behind.

A workspace is only removed once it is older than the longest possible run (plus
a grace period), so a run that is still active is never swept. Generated files
are uploaded to object storage while the run is in progress, so nothing else
accumulates in the execution directory.
*/
func StartSweeper(
	ctx context.Context, cfg *config.Config, logger *slog.Logger,
) {
	workspaceAge := cfg.MaxRunDuration + workspaceGrace
	if cfg.KeepWorkspaces {
		// The operator asked to keep workspaces for inspection.
		workspaceAge = 0
	}

	sweep := func() {
		workspaces := sweepLeftovers(cfg.ExecDir, workspaceAge, time.Now(), logger)
		if workspaces > 0 {
			logger.Info("judge sweep removed leftovers", "workspaces", workspaces)
		}
	}

	sweep()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}

/*
sweepLeftovers removes abandoned run directories and returns how many it
removed. Only directories are touched: the workspace is writable by
submissions, so a symlink planted there must never make the judge delete
something outside ExecDir.
*/
func sweepLeftovers(
	execDir string, workspaceAge time.Duration,
	now time.Time, logger *slog.Logger,
) (workspaces int) {
	// Output archives used to be written here and swept by retention. They are
	// now uploaded to object storage as each case finishes, so the directory is
	// dead weight on an upgraded host: remove it rather than keep publishing
	// space for files nothing serves.
	outputsDir := filepath.Join(execDir, "outputs")
	if _, err := os.Stat(outputsDir); err == nil {
		if err := os.RemoveAll(outputsDir); err != nil {
			logger.Warn("could not remove the obsolete output-archive directory",
				"dir", outputsDir, "error", err,
			)
		}
	}

	if workspaceAge <= 0 {
		return 0
	}

	entries, err := os.ReadDir(execDir)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// commit_/expected_ are run workspaces; pending_ holds generated files
		// waiting for an upload that a crashed run never finished.
		if !strings.HasPrefix(name, "commit_") &&
			!strings.HasPrefix(name, "expected_") &&
			!strings.HasPrefix(name, "pending_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) <= workspaceAge {
			continue
		}
		if err := os.RemoveAll(filepath.Join(execDir, name)); err != nil {
			logger.Warn("could not remove abandoned workspace",
				"dir", name, "error", err,
			)
			continue
		}
		workspaces++
	}

	return workspaces
}
