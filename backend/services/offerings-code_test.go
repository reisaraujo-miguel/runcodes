package services

import (
	"regexp"
	"strings"
	"testing"
)

/*
TestNewEnrollmentCode pins the shape of a generated enrollment code. Every
current and rotated code is six uppercase alphanumerics; enrollment rejects
shorter legacy codes after the rotation migration.
*/
func TestNewEnrollmentCode(t *testing.T) {
	codePattern := regexp.MustCompile(`^[A-Z0-9]{6}$`)

	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		code, err := newEnrollmentCode()
		if err != nil {
			t.Fatalf("newEnrollmentCode: %v", err)
		}

		if !codePattern.MatchString(code) {
			t.Fatalf("newEnrollmentCode() = %q, want six uppercase alphanumerics", code)
		}
		for _, r := range code {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("newEnrollmentCode() = %q, contains %q outside the alphabet", code, r)
			}
		}

		if _, ok := seen[code]; ok {
			t.Fatalf("newEnrollmentCode() returned %q twice in %d draws", code, i+1)
		}
		seen[code] = struct{}{}
	}
}
