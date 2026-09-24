package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

// loadConfig publishes the process configuration the handlers read (the session
// cookie's Secure flag depends on it). Load requires these variables.
func loadConfig(t *testing.T) {
	t.Helper()

	for key, value := range map[string]string{
		"RUNCODES_API_PORT":    "8443",
		"RUNCODES_JWT_SECRET":  "secret",
		"RUNCODES_DB_HOST":     "database",
		"RUNCODES_DB_PORT":     "5432",
		"RUNCODES_DB_USER":     "runcodes",
		"RUNCODES_DB_PASSWORD": "password",
		"RUNCODES_DB_NAME":     "runcodes",
	} {
		t.Setenv(key, value)
	}

	if _, err := config.Load(); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
}

// TestLogOutClearsTheSessionCookie is what makes signing out possible at all: the
// session cookie is HttpOnly, so JavaScript cannot remove it and only the server
// can end the browser's session.
func TestLogOutClearsTheSessionCookie(t *testing.T) {
	loadConfig(t)

	rec := httptest.NewRecorder()
	LogOut(rec, httptest.NewRequest(http.MethodPost, "/api/v1/user/logout", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "jwt" {
		t.Fatalf("cookie name = %q, want jwt", cookie.Name)
	}
	if cookie.Value != "" {
		t.Fatalf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge >= 0 {
		t.Fatalf("MaxAge = %d, want a negative value so the cookie is deleted", cookie.MaxAge)
	}
	if !cookie.Expires.Before(time.Now()) {
		t.Fatalf("Expires = %s, want a time in the past", cookie.Expires)
	}

	// The cleared cookie must keep the attributes of the session cookie, or the
	// browser will refuse to replace it: HttpOnly and Path in particular.
	if !cookie.HttpOnly {
		t.Error("the cleared cookie must be HttpOnly")
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want /", cookie.Path)
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", cookie.SameSite)
	}
}
