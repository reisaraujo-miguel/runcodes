package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestCreateRoutes ensures every route pattern registers without conflicting.
// chi panics on ambiguous patterns, so this covers the authoring endpoints too.
func TestCreateRoutes(t *testing.T) {
	router := chi.NewRouter()
	createRoutes(router)

	want := []string{
		"/healthz",
		"/readyz",
		"/api/v1/user/signup",
		"/api/v1/user/login",
		"/api/v1/user/verify-email",
		"/api/v1/user/resend-verification",
		"/api/v1/user/forgot-password",
		"/api/v1/user/reset-password",
		"/api/v1/user/logout",
		"/api/v1/user/profile",
		"/api/v1/user/password",
		"/api/v1/user/offerings",
		"/api/v1/user/offerings/{id}",
		"/api/v1/user/exercises",
		"/api/v1/settings/public",
		"/api/v1/allowed-file-types",
		"/api/v1/offerings",
		"/api/v1/offerings/enroll",
		"/api/v1/offerings/{id}",
		"/api/v1/offerings/{id}/enrollment",
		"/api/v1/offerings/{id}/members",
		"/api/v1/offerings/{id}/members/{userId}",
		"/api/v1/offerings/{offeringId}/exercises",
		"/api/v1/exercises/{id}",
		"/api/v1/exercises/{id}/submissions",
		"/api/v1/submissions/{id}/source",
		"/api/v1/submissions/{id}/output",
		"/api/v1/submissions/{id}/events",
		"/api/v1/exercises/{id}/test-cases",
		"/api/v1/exercises/{id}/test-cases/{caseId}",
		"/api/v1/exercises/{id}/compilation-files",
		"/api/v1/exercises/{id}/compilation-files/{fileId}",
		"/api/v1/exercises/{id}/attached-files",
		"/api/v1/exercises/{id}/attached-files/{fileId}",
		"/api/v1/admin/users",
		"/api/v1/admin/users/{id}",
		"/api/v1/admin/offerings",
		"/api/v1/admin/offerings/{id}",
		"/api/v1/admin/offerings/{id}/members",
		"/api/v1/admin/settings",
	}

	for _, path := range want {
		if !hasRoute(router, path) {
			t.Errorf("route %q is not registered", path)
		}
	}
}

func hasRoute(router chi.Routes, path string) bool {
	return router.Match(chi.NewRouteContext(), "GET", path) ||
		router.Match(chi.NewRouteContext(), "POST", path) ||
		router.Match(chi.NewRouteContext(), "PUT", path) ||
		router.Match(chi.NewRouteContext(), "DELETE", path)
}

func TestLimitRequestBodyCapsAllNonMultipart(t *testing.T) {
	// The cap must not depend on the Content-Type header: a client can omit it.
	for _, contentType := range []string{"application/json", ""} {
		t.Run("content type "+contentType, func(t *testing.T) {
			var readErr error
			handler := limitRequestBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				_, readErr = io.ReadAll(r.Body)
			}))

			request := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(strings.Repeat("a", maxRequestBodyBytes+1)))
			if contentType != "" {
				request.Header.Set("Content-Type", contentType)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)

			var maxBytesErr *http.MaxBytesError
			if !errors.As(readErr, &maxBytesErr) {
				t.Fatalf("body read error = %v, want *http.MaxBytesError", readErr)
			}
		})
	}
}

func TestLimitRequestBodyLeavesMultipartAlone(t *testing.T) {
	var readErr error
	handler := limitRequestBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))

	body := strings.Repeat("a", maxRequestBodyBytes+1)
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if readErr != nil {
		t.Fatalf("multipart body was capped: %v", readErr)
	}
}
