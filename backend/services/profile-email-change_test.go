package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func expectProfile(mock sqlmock.Sqlmock, email, pending string) {
	mock.ExpectQuery(`SELECT id, name, email, COALESCE\(org_id, ''\), role, confirmed,`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "email", "org_id", "role", "confirmed",
			"code_editor_enabled", "created_at", "pending_email",
		}).AddRow(
			7, "Ada", email, "", "student", true, true, time.Now(), pending,
		))
}

// A profile update that changes the email must not be possible with a stolen
// session alone: the current password is required.
func TestUpdateProfileEmailChangeRequiresCurrentPassword(t *testing.T) {
	mock := newMockDB(t)
	expectProfile(mock, "old@example.com", "")

	_, err := UpdateProfile(context.Background(), 7, &models.UpdateProfileRequest{
		Email: ptrString("new@example.com"),
	})
	if !errors.Is(err, ErrCurrentPasswordRequired) {
		t.Fatalf("UpdateProfile = %v, want ErrCurrentPasswordRequired", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateProfileEmailChangeRejectsWrongPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	mock := newMockDB(t)
	expectProfile(mock, "old@example.com", "")
	mock.ExpectQuery(`SELECT password_hash FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(hash)))

	_, err = UpdateProfile(context.Background(), 7, &models.UpdateProfileRequest{
		Email:           ptrString("new@example.com"),
		CurrentPassword: "wrong horse",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("UpdateProfile = %v, want ErrInvalidCredentials", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateProfileEmailChangeStoresPendingAddress(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

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
	expectProfile(mock, "old@example.com", "")
	mock.ExpectQuery(`SELECT password_hash FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(hash)))
	mock.ExpectQuery(`SELECT id FROM users`).
		WithArgs(7, "new@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(`UPDATE users`).
		WithArgs("Ada", "old@example.com", "", true, "new@example.com", 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO email_verification_tokens`).
		WithArgs(7, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	profile, err := UpdateProfile(context.Background(), 7, &models.UpdateProfileRequest{
		Email:           ptrString("new@example.com"),
		CurrentPassword: "correct horse",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if profile.Email != "old@example.com" {
		t.Fatalf("email = %q, want the old address until verification", profile.Email)
	}
	if profile.PendingEmail != "new@example.com" {
		t.Fatalf("pending email = %q, want new@example.com", profile.PendingEmail)
	}
	if sent.To != "new@example.com" {
		t.Fatalf("verification mail sent to %q, want new@example.com", sent.To)
	}
	if sent.Subject == "" {
		t.Fatal("verification mail has no subject")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
