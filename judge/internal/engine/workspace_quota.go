package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sync"
	"time"
)

// workspaceQuotaPollInterval is how often the judge samples the workspace while
// a container is alive. It is deliberately short: an untrusted process can write
// a lot between samples, so the limits are a safety valve rather than a hard
// filesystem quota.
const workspaceQuotaPollInterval = 250 * time.Millisecond

// errWorkspaceLimitReached is a sentinel that stops the walk as soon as the
// sample is already over a configured limit. Without it a malicious tree could
// make one sample take longer than the run it is supposed to bound.
var errWorkspaceLimitReached = errors.New("workspace limit reached")

// workspaceLimitError is returned when a run exceeds its aggregate workspace
// budget. It is intentionally not a timeoutError: this is a policy/infrastructure
// failure, not the submission's execution budget.
type workspaceLimitError struct {
	usedBytes   int64
	usedEntries int64
	maxBytes    int64
	maxEntries  int64
}

func (e *workspaceLimitError) Error() string {
	return fmt.Sprintf(
		"workspace limit exceeded: %d bytes and %d entries (limits %d bytes, %d entries; 0 means unlimited)",
		e.usedBytes, e.usedEntries, e.maxBytes, e.maxEntries,
	)
}

// workspaceMonitor samples the bind-mounted workspace and kills its container
// when the aggregate size or entry count crosses a configured limit.
type workspaceMonitor struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once

	mu      sync.Mutex
	failure error
}

func (m *workspaceMonitor) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failure
}

func (m *workspaceMonitor) setFailure(err error) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return false
	}
	m.failure = err
	return true
}

// stop cancels the sampler and waits for it to finish. It is idempotent.
func (m *workspaceMonitor) stop() {
	m.stopOnce.Do(func() {
		m.cancel()
		<-m.done
	})
}

// workspaceUsage returns the logical size of regular files and the number of
// filesystem entries below root. Symlinks are counted as entries but never
// followed, and hard links/sparse files are counted conservatively by their
// logical size.
func workspaceUsage(
	root string, maxBytes int64, maxEntries int64,
) (bytes int64, entries int64, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		entries++
		if maxEntries > 0 && entries > maxEntries {
			return errWorkspaceLimitReached
		}
		if !d.Type().IsRegular() {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		bytes += info.Size()
		if maxBytes > 0 && bytes > maxBytes {
			return errWorkspaceLimitReached
		}
		return nil
	})
	if errors.Is(err, errWorkspaceLimitReached) {
		return bytes, entries, errWorkspaceLimitReached
	}
	return bytes, entries, err
}

// startWorkspaceMonitor starts sampling ws.BaseDir until stop is called. A nil
// monitor means both limits are disabled.
func (e *Engine) startWorkspaceMonitor(
	ctx context.Context, dir string, ct RuntimeContainer,
) *workspaceMonitor {
	maxBytes := e.cfg.MaxWorkspaceBytes
	maxEntries := e.cfg.MaxWorkspaceEntries
	if maxBytes <= 0 && maxEntries <= 0 {
		return nil
	}

	logger := e.logger
	if logger == nil {
		logger = slog.Default()
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	m := &workspaceMonitor{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	check := func() bool {
		usedBytes, usedEntries, err := workspaceUsage(dir, maxBytes, maxEntries)
		if errors.Is(err, errWorkspaceLimitReached) {
			limitErr := &workspaceLimitError{
				usedBytes: usedBytes, usedEntries: usedEntries,
				maxBytes: maxBytes, maxEntries: maxEntries,
			}
			if m.setFailure(limitErr) {
				logger.Warn("workspace limit exceeded; killing container",
					"used_bytes", usedBytes, "used_entries", usedEntries,
					"max_bytes", maxBytes, "max_entries", maxEntries,
				)
				if killErr := ct.Kill(context.Background()); killErr != nil {
					logger.Warn("could not kill container after workspace limit",
						"error", killErr,
					)
				}
			}
			return false
		}
		if err != nil {
			// The submission can chmod its own directories to 000 and hide files
			// from the sampler. A permission failure therefore fails closed: the
			// quota cannot be verified, so the run cannot continue. Transient
			// disappearance (a file removed between readdir and stat) is ignored
			// and caught by the next sample if it matters.
			if errors.Is(err, fs.ErrPermission) {
				quotaErr := fmt.Errorf("workspace quota could not be verified: %w", err)
				if m.setFailure(quotaErr) {
					logger.Warn("workspace quota could not be verified; killing container",
						"error", err,
					)
					if killErr := ct.Kill(context.Background()); killErr != nil {
						logger.Warn("could not kill container after workspace quota error",
							"error", killErr,
						)
					}
				}
				return false
			}

			logger.Debug("workspace quota sample failed", "error", err)
			return true
		}

		return true
	}

	go func() {
		defer close(m.done)

		// Check before the first tick so a workspace that was already over the
		// limit during preparation does not get a full poll interval of runtime.
		if !check() {
			return
		}

		ticker := time.NewTicker(workspaceQuotaPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				if !check() {
					return
				}
			}
		}
	}()

	return m
}

// workspaceAwareError prefers a quota failure over the generic log-stream error
// it caused. The caller's deferred cleanup stops the container before stopping
// the monitor, so a background writer is still covered.
func workspaceAwareError(monitor *workspaceMonitor, err error) error {
	if monitor == nil {
		return err
	}
	if limitErr := monitor.Err(); limitErr != nil {
		return limitErr
	}
	return err
}
