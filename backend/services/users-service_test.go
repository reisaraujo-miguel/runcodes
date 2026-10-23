package services

import (
	"context"
	"errors"
	"testing"

	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

// A correct password on an unconfirmed account is refused, and the refusal is
// distinct from a wrong password so the client can offer to resend the link.
func TestLogInRejectsAnUnconfirmedAccount(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name, password_hash, role, confirmed, token_version FROM users`).
		WithArgs("ada@example.com").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "password_hash", "role", "confirmed", "token_version"},
		).AddRow(7, "Ada", string(hash), "student", false, 3))

	_, err = LogIn(context.Background(), &models.LogInRequest{
		Email: "ada@example.com", Password: "correct horse",
	})
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("LogIn = %v, want ErrEmailNotVerified", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogInAcceptsAConfirmedAccount(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name, password_hash, role, confirmed, token_version FROM users`).
		WithArgs("ada@example.com").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "password_hash", "role", "confirmed", "token_version"},
		).AddRow(7, "Ada", string(hash), "student", true, 3))

	claims, err := LogIn(context.Background(), &models.LogInRequest{
		Email: "ada@example.com", Password: "correct horse",
	})
	if err != nil {
		t.Fatalf("LogIn = %v, want nil", err)
	}
	if claims["id"] != 7 || claims["role"] != "student" || claims["token_version"] != 3 {
		t.Fatalf("claims = %+v", claims)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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

func TestCheckEmailExistenceNormalizesEmail(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id FROM users WHERE email = \$1`).
		WithArgs("ada@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := CheckEmailExistence(context.Background(), "  Ada@Example.COM  "); err != nil {
		t.Fatalf("CheckEmailExistence = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogInNormalizesEmail(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name, password_hash, role, confirmed, token_version FROM users`).
		WithArgs("ada@example.com").
		WillReturnRows(sqlmock.NewRows(
			[]string{"id", "name", "password_hash", "role", "confirmed", "token_version"},
		).AddRow(7, "Ada", string(hash), "student", true, 1))

	claims, err := LogIn(context.Background(), &models.LogInRequest{
		Email: "  Ada@Example.COM  ", Password: "correct horse",
	})
	if err != nil {
		t.Fatalf("LogIn = %v, want nil", err)
	}
	if claims["email"] != "ada@example.com" {
		t.Fatalf("claims email = %v, want normalized address", claims["email"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDummyPasswordHashIsUsable(t *testing.T) {
	// A malformed placeholder would make the unknown-account path return
	// immediately and silently reintroduce the timing disclosure.
	if err := bcrypt.CompareHashAndPassword(
		[]byte(dummyPasswordHash), []byte("runcodes-dummy-password"),
	); err != nil {
		t.Fatalf("dummy bcrypt hash is invalid: %v", err)
	}
}
