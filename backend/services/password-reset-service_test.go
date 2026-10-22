package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/mailer"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordResetTokenHashIsNotTheToken(t *testing.T) {
	token := "a-password-reset-token"
	hash := passwordResetTokenHash(token)

	if hash == token {
		t.Fatal("the stored hash is the token itself")
	}
	if len(hash) != 64 {
		t.Fatalf("hash length = %d, want 64 hex characters", len(hash))
	}
	if hash != passwordResetTokenHash(token) {
		t.Fatal("hashing the same token twice produced different values")
	}
	if hash == passwordResetTokenHash(token+"x") {
		t.Fatal("different tokens produced the same hash")
	}
}

// The email carries the only copy of the token; the database must receive its
// hash. This test also pins the link's origin to the configured public URL.
func TestSendPasswordResetEmailStoresOnlyTheHash(t *testing.T) {
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
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WithArgs(7, recordedHash, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := SendPasswordResetEmail(context.Background(), 7, "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("SendPasswordResetEmail = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	if sent.To != "ada@example.com" {
		t.Fatalf("message To = %q", sent.To)
	}
	if sent.Subject != "Redefina sua senha no RunCodes" {
		t.Fatalf("message Subject = %q", sent.Subject)
	}
	linkPattern := regexp.MustCompile(`https://runcodes\.example/reset-password\?token=([A-Za-z0-9_-]+)`)
	match := linkPattern.FindStringSubmatch(sent.Text)
	if match == nil {
		t.Fatalf("password reset link missing from the message:\n%s", sent.Text)
	}
	token := match[1]

	if recordedHash.value == token {
		t.Fatal("the database received the token instead of its hash")
	}
	if recordedHash.value != passwordResetTokenHash(token) {
		t.Fatal("the database hash does not match the emailed token")
	}
	if !regexp.MustCompile(`token=`).MatchString(sent.HTML) {
		t.Fatal("the HTML part has no password reset link")
	}
}

func TestPasswordResetLinkUsesTheConfiguredOrigin(t *testing.T) {
	previousConfig := config.C
	config.C = &config.Config{PublicURL: "https://runcodes.example"}
	t.Cleanup(func() { config.C = previousConfig })

	got := passwordResetLink("abc")
	want := "https://runcodes.example/reset-password?token=abc"
	if got != want {
		t.Fatalf("passwordResetLink = %q, want %q", got, want)
	}
}

// The token is single use: consuming it must fail once it is gone, expired or
// already consumed, without touching the password.
func TestResetPasswordRejectsAnUnusableToken(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WithArgs(passwordResetTokenHash("token")).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
	mock.ExpectRollback()

	err := ResetPassword(context.Background(), "token", "N3w!pass")
	if !errors.Is(err, ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword = %v, want ErrInvalidPasswordResetToken", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetPasswordRejectsAnOversizedTokenBeforeQuerying(t *testing.T) {
	newMockDB(t)

	err := ResetPassword(
		context.Background(), strings.Repeat("x", 129), "N3w!pass",
	)
	if !errors.Is(err, ErrInvalidPasswordResetToken) {
		t.Fatalf("ResetPassword = %v, want ErrInvalidPasswordResetToken", err)
	}
}

func TestRequestPasswordResetSendsToTheAccount(t *testing.T) {
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
	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("ada@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(7, "Ada"))
	mock.ExpectExec(`INSERT INTO password_reset_tokens`).
		WithArgs(7, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := RequestPasswordReset(
		context.Background(), "ada@example.com",
	); err != nil {
		t.Fatalf("RequestPasswordReset = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if sent.To != "ada@example.com" {
		t.Fatalf("message To = %q, want the account address", sent.To)
	}
}

func TestRequestPasswordResetIsSilentForAnUnknownAddress(t *testing.T) {
	previousSend := sendMail
	sendMail = func(context.Context, mailer.Message) error {
		t.Fatal("sent mail for an address that has no account")
		return nil
	}
	t.Cleanup(func() { sendMail = previousSend })

	mock := newMockDB(t)
	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	if err := RequestPasswordReset(
		context.Background(), "nobody@example.com",
	); err != nil {
		t.Fatalf("RequestPasswordReset = %v, want nil", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Expiry is asserted through the value the service stores, so a change to the
// constant has to be deliberate.
func TestPasswordResetTokenTTL(t *testing.T) {
	if PasswordResetTokenTTL != time.Hour {
		t.Fatalf("PasswordResetTokenTTL = %s, want 1h", PasswordResetTokenTTL)
	}
}

// The new password must be stored as a bcrypt hash, never as plaintext.
func TestResetPasswordHashesTheNewPassword(t *testing.T) {
	mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	recorder := &valueRecorder{}
	mock.ExpectExec(`UPDATE users SET password_hash`).
		WithArgs(recorder, 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := ResetPassword(context.Background(), "token", "N3w!pass"); err != nil {
		t.Fatalf("ResetPassword = %v", err)
	}
	if recorder.value == "N3w!pass" {
		t.Fatal("the database received the plaintext password")
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(recorder.value), []byte("N3w!pass"),
	); err != nil {
		t.Fatalf("stored value is not a hash of the new password: %v", err)
	}
}
