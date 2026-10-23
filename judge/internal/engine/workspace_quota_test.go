package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/podman"
)

type quotaFakeContainer struct {
	kills atomic.Int32
}

func (f *quotaFakeContainer) Start(context.Context) error { return nil }
func (f *quotaFakeContainer) Logs(context.Context) *podman.LogStream {
	return nil
}
func (f *quotaFakeContainer) Wait(time.Duration) (int32, error) { return 0, nil }
func (f *quotaFakeContainer) Kill(context.Context) error {
	f.kills.Add(1)
	return nil
}
func (f *quotaFakeContainer) Remove(context.Context) error { return nil }

func TestWorkspaceUsageCountsFilesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.c"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	bytes, entries, err := workspaceUsage(root, 0, 0)
	if err != nil {
		t.Fatalf("workspaceUsage: %v", err)
	}
	if bytes != 5 {
		t.Errorf("bytes = %d, want 5 (the symlink target must not be followed)", bytes)
	}
	if entries != 4 {
		t.Errorf("entries = %d, want 4 (root, file, dir, symlink)", entries)
	}
}

func TestWorkspaceMonitorKillsOnByteLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big"), make([]byte, 32), 0o644); err != nil {
		t.Fatal(err)
	}

	ct := &quotaFakeContainer{}
	e := &Engine{cfg: &config.Config{MaxWorkspaceBytes: 16}}
	monitor := e.startWorkspaceMonitor(context.Background(), root, ct)
	if monitor == nil {
		t.Fatal("monitor is nil")
	}
	defer monitor.stop()

	waitForQuotaFailure(t, monitor)
	var limitErr *workspaceLimitError
	if !errors.As(monitor.Err(), &limitErr) {
		t.Fatalf("error = %v, want workspaceLimitError", monitor.Err())
	}
	if got := ct.kills.Load(); got != 1 {
		t.Errorf("Kill calls = %d, want 1", got)
	}
}

func TestWorkspaceMonitorKillsOnEntryLimit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ct := &quotaFakeContainer{}
	e := &Engine{cfg: &config.Config{MaxWorkspaceEntries: 2}}
	monitor := e.startWorkspaceMonitor(context.Background(), root, ct)
	if monitor == nil {
		t.Fatal("monitor is nil")
	}
	defer monitor.stop()

	waitForQuotaFailure(t, monitor)
	var limitErr *workspaceLimitError
	if !errors.As(monitor.Err(), &limitErr) {
		t.Fatalf("error = %v, want workspaceLimitError", monitor.Err())
	}
}

func TestWorkspaceMonitorDisabled(t *testing.T) {
	e := &Engine{cfg: &config.Config{}}
	ct := &quotaFakeContainer{}
	if monitor := e.startWorkspaceMonitor(context.Background(), t.TempDir(), ct); monitor != nil {
		monitor.stop()
		t.Fatal("monitor started with both limits disabled")
	}
	if got := ct.kills.Load(); got != 0 {
		t.Fatalf("Kill calls = %d, want 0", got)
	}
}

func waitForQuotaFailure(t *testing.T, monitor *workspaceMonitor) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if monitor.Err() != nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("workspace monitor did not report an overage")
}

func TestWorkspaceUsageStopsOnDirectoryEntries(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	_, entries, err := workspaceUsage(root, 0, 2)
	if !errors.Is(err, errWorkspaceLimitReached) {
		t.Fatalf("workspaceUsage error = %v, want errWorkspaceLimitReached", err)
	}
	if entries <= 2 {
		t.Fatalf("entries = %d, want the walk to stop after exceeding 2", entries)
	}
}

func TestWorkspaceMonitorFailsClosedOnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a 000 directory, so the permission path is not reachable")
	}

	root := t.TempDir()
	hidden := filepath.Join(root, "hidden")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hidden, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hidden, 0o755) })

	ct := &quotaFakeContainer{}
	e := &Engine{cfg: &config.Config{MaxWorkspaceBytes: 1}}
	monitor := e.startWorkspaceMonitor(context.Background(), root, ct)
	if monitor == nil {
		t.Fatal("monitor is nil")
	}
	defer monitor.stop()

	waitForQuotaFailure(t, monitor)
	if got := ct.kills.Load(); got != 1 {
		t.Errorf("Kill calls = %d, want 1", got)
	}
}
