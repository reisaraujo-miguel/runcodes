package main

import (
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
