package services

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/mailer"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// valueRecorder accepts any argument and remembers the last string it saw, so a
// test can assert on a value generated inside the function under test.
type valueRecorder struct {
	value string
}

func (r *valueRecorder) Match(value driver.Value) bool {
	s, ok := value.(string)
	if ok {
		r.value = s
	}
	return true
}

func TestVerificationTokenHashIsNotTheToken(t *testing.T) {
	token := "a-verification-token"
	hash := verificationTokenHash(token)

	if hash == token {
		t.Fatal("the stored hash is the token itself")
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64 hex characters", len(hash))
	}
	if hash != verificationTokenHash(token) {
		t.Fatal("hashing the same token twice produced different values")
	}
	if hash == verificationTokenHash(token+"x") {
		t.Fatal("different tokens produced the same hash")
	}
}

func TestVerifyEmailConfirmsTheAccount(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE email_verification_tokens`).
		WithArgs(verificationTokenHash("token")).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET confirmed = TRUE`).
		WithArgs(7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := VerifyEmail(context.Background(), "token"); err != nil {
		t.Fatalf("VerifyEmail = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyEmailRejectsAnUnknownToken(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE email_verification_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()
	mock.ExpectQuery(`SELECT u.confirmed`).
		WillReturnRows(sqlmock.NewRows([]string{"confirmed"}))

	err := VerifyEmail(context.Background(), "unknown-token")
	if !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("VerifyEmail = %v, want ErrInvalidVerificationToken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A second click on the successful link finds a consumed token but a confirmed
// account: that is a success, not an error page.
func TestVerifyEmailIsIdempotentAfterConfirmation(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE email_verification_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()
	mock.ExpectQuery(`SELECT u.confirmed`).
		WillReturnRows(sqlmock.NewRows([]string{"confirmed"}).AddRow(true))

	if err := VerifyEmail(context.Background(), "used-token"); err != nil {
		t.Fatalf("VerifyEmail = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyEmailRejectsAnExpiredTokenForAnUnconfirmedAccount(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE email_verification_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()
	mock.ExpectQuery(`SELECT u.confirmed`).
		WillReturnRows(sqlmock.NewRows([]string{"confirmed"}).AddRow(false))

	err := VerifyEmail(context.Background(), "expired-token")
	if !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("VerifyEmail = %v, want ErrInvalidVerificationToken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The email carries the only copy of the token; the database must receive its
// hash. This test also pins the link's origin to the configured public URL.
func TestSendVerificationEmailStoresOnlyTheHash(t *testing.T) {
	previousConfig := config.C
	config.C = &config.Config{PublicURL: "https://runcodes.example"}
	t.Cleanup(func() { config.C = previousConfig })

	previousSend := sendMail
	var sent mailer.Message
	sendMail = func(_ context.Context, msg mailer.Message) error {
		sent = msg
		return nil
	}
	t.Cleanup(func() { sendMail = previousSend })

	mock := newMockDB(t)
	recordedHash := &valueRecorder{}
	mock.ExpectExec(`INSERT INTO email_verification_tokens`).
		WithArgs(7, recordedHash, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := SendVerificationEmail(context.Background(), 7, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("SendVerificationEmail = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if sent.To != "ada@example.com" {
		t.Fatalf("message To = %q", sent.To)
	}
	if sent.Subject != "Confirme seu email no RunCodes" {
		t.Fatalf("message Subject = %q", sent.Subject)
	}
	linkPattern := regexp.MustCompile(`https://runcodes\.example/verify-email\?token=([A-Za-z0-9_-]+)`)
	match := linkPattern.FindStringSubmatch(sent.Text)
	if match == nil {
		t.Fatalf("verification link missing from the message:\n%s", sent.Text)
	}
	token := match[1]

	if recordedHash.value == token {
		t.Fatal("the database received the token instead of its hash")
	}
	if recordedHash.value != verificationTokenHash(token) {
		t.Fatal("the database hash does not match the emailed token")
	}
	if !regexp.MustCompile(`token=`).MatchString(sent.HTML) {
		t.Fatal("the HTML part has no verification link")
	}
}

func TestVerificationLinkUsesTheConfiguredOrigin(t *testing.T) {
	previousConfig := config.C
	config.C = &config.Config{PublicURL: "https://runcodes.example"}
	t.Cleanup(func() { config.C = previousConfig })

	got := verificationLink("abc")
	want := "https://runcodes.example/verify-email?token=abc"
	if got != want {
		t.Fatalf("verificationLink = %q, want %q", got, want)
	}
}

// Expiry is asserted through the value the service stores, so a change to the
// constant has to be deliberate.
func TestVerificationTokenTTL(t *testing.T) {
	if VerificationTokenTTL != 24*time.Hour {
		t.Fatalf("VerificationTokenTTL = %s, want 24h", VerificationTokenTTL)
	}
}
