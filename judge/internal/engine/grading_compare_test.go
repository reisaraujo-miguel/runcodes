package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runcodes-icmc/judge/internal/model"
)

func TestCompareOutput(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		expected string
		typ      string
		want     model.CaseStatus
	}{
		{"text exact", "hello\nworld\n", "hello\nworld\n", "text", model.CaseCorrect},
		{"text lenient", "HELLO world\n", "hello  WORLD\n", "text", model.CaseBadFormat},
		{"text wrong", "bye\n", "hello\n", "text", model.CaseKilledBySignal},
		{"file equal", "abc", "abc", "file", model.CaseCorrect},
		{"file different", "abc", "abd", "file", model.CaseKilledBySignal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := compareOutput([]byte(tc.user), []byte(tc.expected), tc.typ)
			if got != tc.want {
				t.Fatalf("compareOutput = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestOpenRegularRefusesSymlinks(t *testing.T) {
	dir := t.TempDir()

	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("leaked"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "1.output")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	// The workspace is written by the submission, so a symlinked output must not
	// be followed: doing so would publish whatever file it points at as the
	// submission's own output.
	if _, err := openRegular(link); err == nil {
		t.Fatal("openRegular followed a symlink to a regular file")
	}

	if got := readLimited(link, 1024); got != "" {
		t.Fatalf("readLimited(symlink) = %q, want empty", got)
	}

	if _, err := readRegularBounded(link, 1024); err == nil {
		t.Fatal("readRegularBounded followed a symlink to a regular file")
	}

	// ...and a directory is refused too, rather than read as a huge file.
	if _, err := openRegular(dir); err == nil {
		t.Fatal("openRegular accepted a directory")
	}
}

func TestReadRegularBoundedEnforcesTheLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1.output")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readRegularBounded(path, 6)
	if err != nil || string(got) != "abcdef" {
		t.Fatalf("readRegularBounded(6) = %q, %v", got, err)
	}

	// An oversized output must be an error, not a silently truncated compare.
	if _, err := readRegularBounded(path, 5); err == nil {
		t.Fatal("readRegularBounded accepted a file over the limit")
	}
}

func TestReadLimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := readLimited(path, 3); got != "abc" {
		t.Fatalf("readLimited(max=3) = %q, want abc", got)
	}
	if got := readLimited(path, 0); got != "abcdef" {
		t.Fatalf("readLimited(max=0) = %q, want the whole file", got)
	}
	if got := readLimited(filepath.Join(dir, "missing"), 10); got != "" {
		t.Fatalf("readLimited(missing) = %q, want empty", got)
	}
}
