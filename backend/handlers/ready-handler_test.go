package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReady(t *testing.T) {
	restore := stubReadinessDependencies(t, nil, nil)
	defer restore()

	rec := httptest.NewRecorder()
	Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	for key, want := range map[string]string{"status": "ok", "database": "ok", "judge": "ok"} {
		if body[key] != want {
			t.Errorf("%s = %q, want %q", key, body[key], want)
		}
	}
}

func TestReadyDatabaseFailure(t *testing.T) {
	restore := stubReadinessDependencies(t, errors.New("connection refused"), nil)
	defer restore()

	rec := httptest.NewRecorder()
	Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body["database"] != "unavailable" || body["status"] != "unavailable" {
		t.Fatalf("body = %v, want database/status unavailable", body)
	}
}

func TestReadyJudgeFailureDoesNotFailReadiness(t *testing.T) {
	restore := stubReadinessDependencies(t, nil, errors.New("judge down"))
	defer restore()

	rec := httptest.NewRecorder()
	Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body["judge"] != "unavailable" || body["status"] != "ok" {
		t.Fatalf("body = %v, want judge unavailable with status ok", body)
	}
}

func stubReadinessDependencies(
	t *testing.T, databaseErr, judgeErr error,
) func() {
	t.Helper()

	previousDatabase := databasePing
	previousJudge := judgePing
	databasePing = func(context.Context) error { return databaseErr }
	judgePing = func(context.Context) error { return judgeErr }

	return func() {
		databasePing = previousDatabase
		judgePing = previousJudge
	}
}
