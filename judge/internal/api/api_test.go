package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

type fakeRuntime struct{ err error }

func (f fakeRuntime) Ready(context.Context) error { return f.err }

type fakeArtifacts struct{ path string }

func (f fakeArtifacts) ArtifactPath(int64) string { return f.path }

func newServer(db fakeDB, rt fakeRuntime, art fakeArtifacts, token string) (*Server, *events.Hub, *int32) {
	hub := events.New(time.Minute)
	var woke int32
	srv := New(
		&config.Config{AuthToken: token},
		db, rt, art, hub,
		func() { atomic.AddInt32(&woke, 1) },
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return srv, hub, &woke
}

func TestHealth(t *testing.T) {
	srv, _, _ := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{}, "")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name string
		db   error
		rt   error
		want int
	}{
		{"ok", nil, nil, http.StatusOK},
		{"database down", errors.New("db"), nil, http.StatusServiceUnavailable},
		{"podman down", nil, errors.New("podman"), http.StatusServiceUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _, _ := newServer(fakeDB{tc.db}, fakeRuntime{tc.rt}, fakeArtifacts{}, "")
			rec := httptest.NewRecorder()
			srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != tc.want {
				t.Fatalf("code = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestAuth(t *testing.T) {
	srv, _, woke := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{}, "secret")
	router := srv.Router()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs/1", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: code = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/1", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: code = %d, want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/runs/1", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("good token: code = %d, want 202", rec.Code)
	}
	if got := atomic.LoadInt32(woke); got != 1 {
		t.Fatalf("wake calls = %d, want 1", got)
	}
}

func TestWakeInvalidID(t *testing.T) {
	srv, _, _ := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{}, "")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs/abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestEventsStream(t *testing.T) {
	srv, hub, _ := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{}, "tok")
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/runs/9/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer tok")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}

	// The hub retains frames, so publishing here is safe regardless of the
	// exact subscribe timing; `finished` closes the stream.
	hub.Status(9, "compiling", time.Now())
	hub.Finished(9, "completed", 1, 100, "", "", time.Now(), time.Now())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"event: status", "event: finished", `"status":"compiling"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in stream:\n%s", want, text)
		}
	}
}

func TestOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1.zip")
	if err := os.WriteFile(path, []byte("PK\x03\x04payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, _, _ := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{path: path}, "")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/runs/1/output", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "payload") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestOutputMissing(t *testing.T) {
	srv, _, _ := newServer(fakeDB{}, fakeRuntime{}, fakeArtifacts{path: filepath.Join(t.TempDir(), "nope.zip")}, "")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/runs/1/output", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}
