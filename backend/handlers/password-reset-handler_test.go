package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runcodes-icmc/runcodes/database"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// newHandlerMockDB swaps the package-level database handle for a sqlmock one
// and restores it when the test finishes.
func newHandlerMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		db.Close()
	})

	return mock
}

func postJSON(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/user/password-reset", strings.NewReader(body),
	)
	handler(rec, req)
	return rec
}

func TestForgotPasswordRejectsAnInvalidEmail(t *testing.T) {
	loadConfig(t)

	for _, body := range []string{
		`{"email":""}`,
		`{"email":"not-an-address"}`,
		`not-json`,
	} {
		rec := postJSON(ForgotPassword, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want %d",
				body, rec.Code, http.StatusBadRequest)
		}
	}
}

// The endpoint must not reveal whether an address has an account, so an
// unknown one still gets the accepted response and no mail.
func TestForgotPasswordAcceptsAnUnknownAddress(t *testing.T) {
	mock := newHandlerMockDB(t)
	mock.ExpectQuery(`SELECT id, name FROM users WHERE email`).
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	rec := postJSON(ForgotPassword, `{"email":"nobody@example.com"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetPasswordReturnsNoContentOnSuccess(t *testing.T) {
	mock := newHandlerMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE password_reset_tokens`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET password_hash`).
		WithArgs(sqlmock.AnyArg(), 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	rec := postJSON(ResetPassword, `{"token":"token","password":"N3w!pass","password_confirmation":"N3w!pass"}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResetPasswordValidatesTheRequest(t *testing.T) {
	loadConfig(t)

	tests := []struct {
		name string
		body string
	}{
		{"missing token", `{"password":"N3w!pass","password_confirmation":"N3w!pass"}`},
		{"mismatched passwords", `{"token":"token","password":"N3w!pass","password_confirmation":"N3w!pass2"}`},
		{"weak password", `{"token":"token","password":"weak","password_confirmation":"weak"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postJSON(ResetPassword, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}
