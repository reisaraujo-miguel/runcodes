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
	outputs := filepath.Join(root, "outputs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	newTime := time.Now()

	write := func(path string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}

	write(filepath.Join(outputs, "1.zip"), oldTime)
	write(filepath.Join(outputs, "2.zip"), newTime)
	write(filepath.Join(outputs, "keep.txt"), oldTime)

	oldCommit := filepath.Join(root, "commit_1")
	oldExpected := filepath.Join(root, "expected_1")
	newCommit := filepath.Join(root, "commit_2")
	for _, dir := range []string{oldCommit, oldExpected, newCommit} {
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
	if err := os.Chtimes(newCommit, newTime, newTime); err != nil {
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
	archives, workspaces := sweepLeftovers(
		root, 7*24*time.Hour, 35*time.Minute, time.Now(), logger,
	)

	if archives != 1 {
		t.Errorf("archives removed = %d, want 1", archives)
	}
	if workspaces != 2 {
		t.Errorf("workspaces removed = %d, want 2", workspaces)
	}
	if _, err := os.Stat(filepath.Join(outputs, "2.zip")); err != nil {
		t.Errorf("recent archive was removed: %v", err)
	}
	if _, err := os.Stat(newCommit); err != nil {
		t.Errorf("recent workspace was removed: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("symlink target was removed: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink was removed: %v", err)
	}
}

func TestSweepLeftoversKeepsArchivesWhenDisabled(t *testing.T) {
	root := t.TempDir()
	outputs := filepath.Join(root, "outputs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(outputs, "1.zip")
	if err := os.WriteFile(archive, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(archive, old, old); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	archives, _ := sweepLeftovers(root, 0, time.Hour, time.Now(), logger)
	if archives != 0 {
		t.Fatalf("archives removed with retention disabled = %d", archives)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive removed with retention disabled: %v", err)
	}
}
