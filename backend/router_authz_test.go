package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

func newAuthzRouter(t *testing.T) *chi.Mux {
	t.Helper()

	previousConfig := config.C
	config.C = &config.Config{JWTSecret: "test-secret"}
	t.Cleanup(func() { config.C = previousConfig })

	if err := validation.SetupJWT(); err != nil {
		t.Fatalf("SetupJWT: %v", err)
	}

	router := chi.NewRouter()
	createRoutes(router)
	return router
}

func newAuthzDB(t *testing.T) sqlmock.Sqlmock {
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

func signedSession(t *testing.T, id float64, role string, version float64) string {
	t.Helper()

	claims := map[string]any{
		"id":            id,
		"name":          "Ada",
		"email":         "ada@example.com",
		"role":          role,
		"token_version": version,
	}
	jwtauth.SetExpiryIn(claims, time.Hour)

	_, token, err := validation.TokenAuth.Encode(claims)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return token
}

func TestAdminRouteRequiresAuthentication(t *testing.T) {
	router := newAuthzRouter(t)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAdminRouteRejectsNonAdminSessions(t *testing.T) {
	for _, role := range []string{"student", "professor"} {
		t.Run(role, func(t *testing.T) {
			router := newAuthzRouter(t)
			mock := newAuthzDB(t)
			mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
				WithArgs(7).
				WillReturnRows(sqlmock.NewRows(
					[]string{"confirmed", "role", "token_version"},
				).AddRow(true, role, 1))

			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			request.AddCookie(&http.Cookie{
				Name:  "jwt",
				Value: signedSession(t, 7, role, 1),
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStaleTokenVersionIsRejected(t *testing.T) {
	router := newAuthzRouter(t)
	mock := newAuthzDB(t)
	mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"confirmed", "role", "token_version"},
		).AddRow(true, "admin", 4))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signedSession(t, 7, "admin", 3),
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
