// Package cmp ports the legacy engine's output comparison functions
// (`rcc/cmp.py`) to Go.
package cmp

import (
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// splitLines mimics Python's file iteration: a trailing newline does not
// produce an extra empty line, and an empty file has no lines.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func readText(fname string) ([]string, bool) {
	raw, err := os.ReadFile(fname)
	if err != nil || !utf8.Valid(raw) {
		return nil, false
	}
	return splitLines(string(raw)), true
}

// TextEqual is the strict comparison: line counts must match and each line must
// be equal after stripping trailing whitespace.
func TextEqual(fnameA, fnameB string) bool {
	a, ok := readText(fnameA)
	if !ok {
		return false
	}
	b, ok := readText(fnameB)
	if !ok {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if trimRightSpace(a[i]) != trimRightSpace(b[i]) {
			return false
		}
	}
	return true
}

// TextLenient is the "bad formatted output" comparison: blank lines are
// skipped and, when a line differs, whitespace-separated tokens are compared
// case-insensitively.
func TextLenient(fnameA, fnameB string) bool {
	a, ok := readText(fnameA)
	if !ok {
		return false
	}
	b, ok := readText(fnameB)
	if !ok {
		return false
	}

	i, j := 0, 0
	for i < len(a) && j < len(b) {
		for i < len(a) && strings.TrimSpace(a[i]) == "" {
			i++
		}
		if i >= len(a) {
			return false
		}
		for j < len(b) && strings.TrimSpace(b[j]) == "" {
			j++
		}
		if j >= len(b) {
			return false
		}
		lineA, lineB := a[i], b[j]
		i++
		j++
		if lineA != lineB {
			tokensA := strings.Fields(strings.ToLower(lineA))
			tokensB := strings.Fields(strings.ToLower(lineB))
			if len(tokensA) != len(tokensB) {
				return false
			}
			for k := range tokensA {
				if tokensA[k] != tokensB[k] {
					return false
				}
			}
		}
	}
	return i >= len(a) && j >= len(b)
}

// NumberEqual compares tokens, allowing a tolerance between numeric tokens.
func NumberEqual(fnameA, fnameB string, absError float64) bool {
	if absError < 0 {
		return false
	}
	a, ok := readText(fnameA)
	if !ok {
		return false
	}
	b, ok := readText(fnameB)
	if !ok {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		tokensA := strings.Fields(a[i])
		tokensB := strings.Fields(b[i])
		if len(tokensA) != len(tokensB) {
			return false
		}
		for k := range tokensA {
			fa, okA := parseFloat(tokensA[k])
			fb, okB := parseFloat(tokensB[k])
			if okA && okB {
				diff := fa - fb
				if diff < 0 {
					diff = -diff
				}
				if diff > absError {
					return false
				}
			} else if tokensA[k] != tokensB[k] {
				return false
			}
		}
	}
	return true
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
