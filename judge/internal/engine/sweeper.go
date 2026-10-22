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

// sweepInterval is how often the judge looks for expired archives and orphaned
// workspaces. Hourly is frequent enough for a 7-day archive retention and cheap
// enough to ignore.
const sweepInterval = time.Hour

// workspaceGrace is added to the maximum run duration before a workspace is
// considered abandoned: a run that is still active must never be swept.
const workspaceGrace = 5 * time.Minute

/*
StartSweeper reclaims generated output archives and abandoned workspaces, then
repeats hourly until ctx is cancelled. The initial sweep also covers whatever a
previous judge process left behind.

Workspaces are only removed once they are older than the longest possible run
(plus a grace period); archives follow the configured ArtifactTTL, where 0 keeps
them forever.
*/
func StartSweeper(
	ctx context.Context, cfg *config.Config, logger *slog.Logger,
) {
	workspaceAge := cfg.MaxRunDuration + workspaceGrace
	if cfg.KeepWorkspaces {
		// The operator asked to keep workspaces for inspection; only archives are
		// swept.
		workspaceAge = 0
	}

	sweep := func() {
		archives, workspaces := sweepLeftovers(
			cfg.ExecDir, cfg.ArtifactTTL, workspaceAge, time.Now(), logger,
		)
		if archives > 0 || workspaces > 0 {
			logger.Info("judge sweep removed leftovers",
				"archives", archives,
				"workspaces", workspaces,
			)
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
sweepLeftovers removes expired output archives and abandoned run directories. It
returns how many of each were removed. Only regular files and directories are
touched: the workspace is writable by submissions, so a symlink planted there
must never make the judge delete something outside ExecDir.
*/
func sweepLeftovers(
	execDir string, artifactTTL, workspaceAge time.Duration,
	now time.Time, logger *slog.Logger,
) (archives, workspaces int) {
	outputsDir := filepath.Join(execDir, "outputs")
	if entries, err := os.ReadDir(outputsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if artifactTTL > 0 && now.Sub(info.ModTime()) > artifactTTL {
				if err := os.Remove(filepath.Join(outputsDir, entry.Name())); err != nil {
					logger.Warn("could not remove expired output archive",
						"name", entry.Name(), "error", err,
					)
					continue
				}
				archives++
			}
		}
	}

	if workspaceAge <= 0 {
		return archives, workspaces
	}

	entries, err := os.ReadDir(execDir)
	if err != nil {
		return archives, workspaces
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "commit_") && !strings.HasPrefix(name, "expected_") {
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

	return archives, workspaces
}
