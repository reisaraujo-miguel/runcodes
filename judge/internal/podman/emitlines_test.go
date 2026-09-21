package podman

import (
	"strings"
	"testing"
)

// collect runs emitLines over the given chunks and returns the emitted lines.
func collect(chunks ...string) []string {
	in := make(chan string, len(chunks))
	for _, c := range chunks {
		in <- c
	}
	close(in)

	out := make(chan string, 1024)
	done := make(chan struct{})
	var lines []string
	go func() {
		defer close(done)
		for line := range out {
			lines = append(lines, line)
		}
	}()

	emitLines(in, out)
	close(out)
	<-done

	return lines
}

func TestEmitLinesSplitsAndJoinsChunks(t *testing.T) {
	// Milestones are matched as exact lines, so splitting must not depend on how
	// the runtime chunks the stream.
	got := collect("run.st", "art\ncompilation.done\npartial")
	want := []string{"run.start", "compilation.done", "partial"}

	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEmitLinesStripsCarriageReturns(t *testing.T) {
	got := collect("run.start\r\n")
	if len(got) != 1 || got[0] != "run.start" {
		t.Fatalf("got %q, want [run.start]", got)
	}
}

func TestEmitLinesPassesEmptyLinesThrough(t *testing.T) {
	// A blank line is emitted as an empty line, matching the previous behaviour.
	// It cannot disturb milestone matching, which compares whole lines.
	got := collect("a\n\nb\n")
	if len(got) != 3 || got[0] != "a" || got[1] != "" || got[2] != "b" {
		t.Fatalf("got %q, want [a \"\" b]", got)
	}
}

func TestEmitLinesDoesNotEmitATrailingNewline(t *testing.T) {
	// A final newline terminates the previous line; it does not start a new one.
	got := collect("a\n")
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %q, want [a]", got)
	}
}

func TestEmitLinesBoundsAnEndlessLine(t *testing.T) {
	// A program that never writes a newline must not be able to grow the buffer
	// without limit: it gets one truncated line and the rest is discarded.
	got := collect(strings.Repeat("a", maxLineBytes*3))

	if len(got) != 1 {
		t.Fatalf("got %d lines, want 1", len(got))
	}
	if len(got[0]) > maxLineBytes+len(truncatedMarker) {
		t.Fatalf("emitted line is %d bytes, want at most %d", len(got[0]), maxLineBytes+len(truncatedMarker))
	}
	if !strings.HasSuffix(got[0], truncatedMarker) {
		t.Fatalf("expected the line to be marked as truncated: %q", got[0])
	}
}

func TestEmitLinesBoundsAnOverlongLineBeforeItsNewline(t *testing.T) {
	// The over-long line is reported once, and its newline does not produce a
	// second (empty) line.
	got := collect(strings.Repeat("x", maxLineBytes+1) + "\nnext\n")

	if len(got) != 2 {
		t.Fatalf("got %q, want 2 lines", got)
	}
	if !strings.HasSuffix(got[0], truncatedMarker) {
		t.Fatalf("first line not marked truncated: %q", got[0])
	}
	if got[1] != "next" {
		t.Fatalf("second line = %q, want next", got[1])
	}
}

func TestEmitLinesKeepsALineAtTheBound(t *testing.T) {
	// Exactly at the bound is fine and must not be marked as truncated.
	line := strings.Repeat("y", maxLineBytes)
	got := collect(line + "\n")

	if len(got) != 1 || got[0] != line {
		t.Fatalf("a line of exactly maxLineBytes should pass through unchanged")
	}
}
