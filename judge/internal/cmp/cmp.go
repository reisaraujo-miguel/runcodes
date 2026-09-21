// Package cmp provides functions to compare text files in different ways,
// including strict comparison, lenient comparison, and numeric comparison with
// a specified tolerance.

package cmp

import (
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// splitLines splits the content of a text file into lines, removing the last line if it is empty.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}

	// Split the content into lines using newline as the delimiter.
	lines := strings.Split(content, "\n")

	// Remove the last line if it is empty.
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// readText reads the content of a text file and returns its lines as a slice of strings.
// It returns false if the file cannot be read or if the content is not valid UTF-8.
func readText(fname string) ([]string, bool) {
	raw, err := os.ReadFile(fname)
	if err != nil || !utf8.Valid(raw) {
		return nil, false
	}

	return splitLines(string(raw)), true
}

// TextEqual is a strict comparison: line counts must match and each line must
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

	// Check if the number of lines in both files is the same.
	if len(a) != len(b) {
		return false
	}

	// Compare each line after trimming trailing whitespace.
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

	// Loop through both files, skipping empty lines and comparing non-empty lines.
	for i < len(a) && j < len(b) {

		// Skip empty lines in the first file.
		for i < len(a) && strings.TrimSpace(a[i]) == "" {
			i++
		}

		if i >= len(a) {
			return false
		}

		// Skip empty lines in the second file.
		for j < len(b) && strings.TrimSpace(b[j]) == "" {
			j++
		}

		if j >= len(b) {
			return false
		}

		// Compare the current lines from both files.
		lineA, lineB := a[i], b[j]
		if lineA != lineB {
			tokensA := strings.Fields(strings.ToLower(lineA))
			tokensB := strings.Fields(strings.ToLower(lineB))

			// If the number of tokens in the lines is different, they are not equal.
			if len(tokensA) != len(tokensB) {
				return false
			}

			// Compare each token in the lines.
			for k := range tokensA {
				if tokensA[k] != tokensB[k] {
					return false
				}
			}
		}

		i++
		j++
	}

	// Skip any trailing blank lines so an extra blank line at the end of one
	// file does not make otherwise equal outputs differ.
	for i < len(a) && strings.TrimSpace(a[i]) == "" {
		i++
	}
	for j < len(b) && strings.TrimSpace(b[j]) == "" {
		j++
	}

	// return true if both files have been fully processed, indicating they are equal.
	return i >= len(a) && j >= len(b)
}

// NumberEqual compares tokens, allowing a tolerance between numeric tokens.
func NumberEqual(fnameA, fnameB string, absError float64) bool {
	// Return false if the absolute error is negative, as it is not a valid comparison.
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

	// Check if the number of lines in both files is the same.
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		tokensA := strings.Fields(a[i])
		tokensB := strings.Fields(b[i])

		// If the number of tokens in the lines is different, they are not equal.
		if len(tokensA) != len(tokensB) {
			return false
		}

		// Compare each token in the lines, allowing for numeric tolerance.
		for k := range tokensA {
			fa, okA := parseFloat(tokensA[k])
			fb, okB := parseFloat(tokensB[k])

			// If both tokens are numbers, compare them with the specified tolerance.
			if okA && okB {
				diff := fa - fb
				// Take the absolute value of the difference.
				if diff < 0 {
					diff = -diff
				}
				// If the difference exceeds the allowed absolute error, they are not equal.
				if diff > absError {
					return false
				}
			} else if tokensA[k] != tokensB[k] { // If either token is not a number, compare them as strings.
				return false
			}
		}
	}

	return true
}

// trimRightSpace removes trailing whitespace from a string.
func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

// parseFloat attempts to parse a string as a float64. It returns the parsed float and a boolean indicating success.
func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}

	return f, true
}
