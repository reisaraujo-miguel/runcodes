package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/runcodes-icmc/judge/internal/model"
)

func TestCompareOutput(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

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
			user := write("user_"+tc.name, tc.user)
			expected := write("expected_"+tc.name, tc.expected)
			if got := compareOutput(user, expected, tc.typ); got != tc.want {
				t.Fatalf("compareOutput = %s, want %s", got, tc.want)
			}
		})
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
