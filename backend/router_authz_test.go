package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
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

// adminSessionRequest builds a request authenticated as an administrator. The
// middleware re-reads the account on every request, so each caller supplies the
// sqlmock row for it.
// adminSessionRequestWithRole signs a session for one account with an explicit
// platform role.
func adminSessionRequestWithRole(
	t *testing.T, method, path, body, role string,
) *http.Request {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signedSession(t, 7, role, 1),
	})

	return request
}

func adminSessionRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)
	request.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signedSession(t, 7, "admin", 1),
	})

	return request
}

func TestAdminAlertsRequireAdminRole(t *testing.T) {
	router := newAuthzRouter(t)
	mock := newAuthzDB(t)
	mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"confirmed", "role", "token_version"},
		).AddRow(true, "student", 1))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/alerts", nil)
	request.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signedSession(t, 7, "student", 1),
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminListAlerts(t *testing.T) {
	router := newAuthzRouter(t)
	mock := newAuthzDB(t)
	mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"confirmed", "role", "token_version"},
		).AddRow(true, "admin", 1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM system_alerts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`FROM system_alerts a`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "created_at", "severity", "code", "source", "commit_id",
			"message", "details", "acknowledged_at", "acknowledged_by",
			"acknowledged_by_name",
		}).AddRow(
			int64(3), time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
			"critical", "startup_timeout", "judge", int64(6),
			"the container did not start", nil, nil, nil, "",
		))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, adminSessionRequest(t, http.MethodGet, "/api/v1/admin/alerts"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`"open_count":1`, `"code":"startup_timeout"`, `"commit_id":6`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body is missing %s: %s", want, body)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminAcknowledgeMissingAlert(t *testing.T) {
	router := newAuthzRouter(t)
	mock := newAuthzDB(t)
	mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"confirmed", "role", "token_version"},
		).AddRow(true, "admin", 1))
	mock.ExpectExec(`UPDATE system_alerts`).
		WithArgs(int64(404), 7).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(int64(404)).
		WillReturnError(sql.ErrNoRows)

	response := httptest.NewRecorder()
	router.ServeHTTP(response,
		adminSessionRequest(t, http.MethodPost, "/api/v1/admin/alerts/404/acknowledge"))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// The bulletin board is class material: it requires a session, and the class
// access rules are enforced by the service rather than by a role group (a
// monitor posts, a student comments, and both are signed-in users only).
func TestBulletinRoutesRequireAuthentication(t *testing.T) {
	router := newAuthzRouter(t)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/offerings/1/bulletin", ""},
		{http.MethodPost, "/api/v1/offerings/1/bulletin/posts", `{"title":"x","body":"y"}`},
		{http.MethodPost, "/api/v1/offerings/1/bulletin/posts/2/comments", `{"body":"y"}`},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status = %d, want %d",
				tc.method, tc.path, response.Code, http.StatusUnauthorized)
		}
	}
}

func TestStudentCannotPostToTheBulletinBoard(t *testing.T) {
	router := newAuthzRouter(t)
	mock := newAuthzDB(t)
	mock.ExpectQuery(`SELECT confirmed, role::text, token_version FROM users`).
		WithArgs(7).
		WillReturnRows(sqlmock.NewRows(
			[]string{"confirmed", "role", "token_version"},
		).AddRow(true, "student", 1))
	mock.ExpectQuery(`SELECT id, name, end_date`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "end_date", "description", "enrollment_code",
			"owner_id", "visible_to_enroll",
		}).AddRow(int64(4), "Algoritmos", time.Now().Add(time.Hour), "", "", int64(9), true))
	mock.ExpectQuery(`SELECT COALESCE\(role::text`).
		WithArgs(int64(4), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "banned"}).
			AddRow("student", false))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, adminSessionRequestWithRole(t, http.MethodPost,
		"/api/v1/offerings/4/bulletin/posts", `{"title":"Oi","body":"Posso?"}`, "student"))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
