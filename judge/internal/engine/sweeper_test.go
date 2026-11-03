package engine

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepLeftovers(t *testing.T) {
	root := t.TempDir()

	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	newTime := time.Now()

	oldCommit := filepath.Join(root, "commit_1")
	oldExpected := filepath.Join(root, "expected_1")
	oldPending := filepath.Join(root, "pending_1")
	newCommit := filepath.Join(root, "commit_2")
	for _, dir := range []string{oldCommit, oldExpected, oldPending, newCommit} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(oldCommit, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldExpected, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldPending, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newCommit, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	// An archive from the previous layout is reclaimed: nothing serves it any
	// more and no retention would ever remove it.
	outputs := filepath.Join(root, "outputs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputs, "1.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A symlink named like an old workspace must not be followed or removed.
	link := filepath.Join(root, "commit_3")
	target := filepath.Join(root, "outside")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workspaces := sweepLeftovers(root, 35*time.Minute, time.Now(), logger)

	if workspaces != 3 {
		t.Errorf("workspaces removed = %d, want 3 (two workspaces and a pending area)", workspaces)
	}
	if _, err := os.Stat(oldPending); !os.IsNotExist(err) {
		t.Errorf("abandoned pending uploads were kept, stat error: %v", err)
	}
	if _, err := os.Stat(newCommit); err != nil {
		t.Errorf("recent workspace was removed: %v", err)
	}
	if _, err := os.Stat(outputs); !os.IsNotExist(err) {
		t.Errorf("obsolete output-archive directory was kept, stat error: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("symlink target was removed: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink was removed: %v", err)
	}
}
