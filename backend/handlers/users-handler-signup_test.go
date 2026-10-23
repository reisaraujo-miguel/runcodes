package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runcodes-icmc/runcodes/database"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

// TestSignUpDoesNotDiscloseAnExistingEmail pins the enumeration fix: a duplicate
// address gets the same 201 as a new signup, not a 409.
func TestSignUpDoesNotDiscloseAnExistingEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		db.Close()
	})

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO users \(name, email, password_hash\) VALUES`).
		WithArgs("Ada", "ada@example.com", sqlmock.AnyArg()).
		WillReturnError(&pq.Error{Code: pqerror.UniqueViolation})
	mock.ExpectRollback()

	body := `{"name":"Ada","email":"Ada@Example.COM","password":"S3cure!pass","password_confirmation":"S3cure!pass"}`
	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/user/signup", strings.NewReader(body),
	)
	rec := httptest.NewRecorder()

	SignUp(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
