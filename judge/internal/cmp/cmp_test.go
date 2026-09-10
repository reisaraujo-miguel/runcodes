package cmp

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTextEqual(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"identical", "hello\nworld\n", "hello\nworld\n", true},
		{"trailing whitespace ignored", "hello  \nworld\t\n", "hello\nworld\n", true},
		{"missing final newline", "hello\nworld", "hello\nworld\n", true},
		{"different line count", "hello\n", "hello\nworld\n", false},
		{"different content", "hello\n", "bye\n", false},
		{"empty files", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := write(t, dir, "a.txt", tc.a)
			b := write(t, dir, "b.txt", tc.b)
			if got := TextEqual(a, b); got != tc.expected {
				t.Fatalf("TextEqual = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestTextLenient(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"blank lines skipped", "hello\n\nworld\n", "hello\nworld\n", true},
		{"case insensitive tokens", "HELLO world\n", "hello  WORLD\n", true},
		{"token mismatch", "hello there\n", "hello world\n", false},
		{"token count mismatch", "hello there\n", "hello\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := write(t, dir, "a.txt", tc.a)
			b := write(t, dir, "b.txt", tc.b)
			if got := TextLenient(a, b); got != tc.expected {
				t.Fatalf("TextLenient = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNumberEqual(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a.txt", "1.0000001 2\n")
	b := write(t, dir, "b.txt", "1.0 2\n")
	if !NumberEqual(a, b, 0.001) {
		t.Fatal("expected numbers within tolerance to match")
	}
	if NumberEqual(a, b, 0.00000001) {
		t.Fatal("expected numbers outside tolerance not to match")
	}
}
