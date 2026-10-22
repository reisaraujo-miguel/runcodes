package services

import (
	"regexp"
	"testing"
)

/*
TestIDToCode pins the shape of a generated enrollment code. New offerings get
six characters; the four-character format is reserved for codes of older
offerings, which enrollment still accepts.
*/
func TestIDToCode(t *testing.T) {
	codePattern := regexp.MustCompile(`^[A-Z0-9]{6}$`)

	const idCount = int64(20_000)
	seen := make(map[string]int64, idCount)
	for id := int64(0); id < idCount; id++ {
		code := IDToCode(id)

		if !codePattern.MatchString(code) {
			t.Fatalf("IDToCode(%d) = %q, want six uppercase alphanumerics", id, code)
		}
		if previous, ok := seen[code]; ok {
			t.Fatalf("IDToCode(%d) and IDToCode(%d) both = %q", previous, id, code)
		}
		seen[code] = id

		if again := IDToCode(id); again != code {
			t.Fatalf("IDToCode(%d) = %q then %q, want deterministic", id, code, again)
		}
	}
}

/*
TestEncodePadsToCodeLength keeps the generated codes fixed-width, so an early
offering ID does not produce a shorter code that could be mistaken for a legacy
four-character one.
*/
func TestEncodePadsToCodeLength(t *testing.T) {
	if got := encode(0); got != "AAAAAA" {
		t.Fatalf("encode(0) = %q, want %q", got, "AAAAAA")
	}
}
