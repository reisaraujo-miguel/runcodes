package services

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerifyBcryptPassword(t *testing.T) {
	ctx := context.Background()

	// MinCost keeps the test fast; the comparison does not care about the cost.
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	if err := verifyBcryptPassword(ctx, "correct horse", string(hash)); err != nil {
		t.Fatalf("correct password rejected: %v", err)
	}

	if err := verifyBcryptPassword(ctx, "wrong horse", string(hash)); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password = %v, want ErrInvalidCredentials", err)
	}
}

// A hash the library cannot parse is the state the seeded admin account is left
// in (see database/schema/03-seed-data.sql): it must be a failed login, not a
// 500 — otherwise the placeholder is an outage that also tells the caller which
// account is locked.
func TestVerifyBcryptPasswordRejectsAnUnusableHash(t *testing.T) {
	ctx := context.Background()

	for _, stored := range []string{"", "!", "not-a-hash", "legacy-sha1$deadbeef"} {
		err := verifyBcryptPassword(ctx, "anything", stored)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("stored %q = %v, want ErrInvalidCredentials", stored, err)
		}
		if errors.Is(err, ErrServer) {
			t.Errorf("stored %q was reported as a server error", stored)
		}
	}
}
