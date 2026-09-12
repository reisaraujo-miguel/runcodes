package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJudgeBaseURL(t *testing.T) {
	t.Setenv(judgeURLEnv, "http://example.test:9000/")
	if got := judgeBaseURL(); got != "http://example.test:9000" {
		t.Fatalf("judgeBaseURL() = %q", got)
	}

	t.Setenv(judgeURLEnv, "")
	if got := judgeBaseURL(); got != defaultJudgeURL {
		t.Fatalf("judgeBaseURL() default = %q", got)
	}
}

func TestJudgeReady(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(judgeURLEnv, server.URL)

	if err := JudgeReady(context.Background()); err != nil {
		t.Fatalf("JudgeReady returned error: %v", err)
	}
	if path != "/readyz" {
		t.Fatalf("expected /readyz, got %q", path)
	}
}

func TestJudgeReadyErrors(t *testing.T) {
	t.Run("not ok status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		t.Setenv(judgeURLEnv, server.URL)

		if err := JudgeReady(context.Background()); err == nil {
			t.Fatal("expected an error for a 503 readiness response")
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		t.Setenv(judgeURLEnv, "http://127.0.0.1:1")
		if err := JudgeReady(context.Background()); err == nil {
			t.Fatal("expected an error for an unreachable judge")
		}
	})
}

func TestWakeJudge(t *testing.T) {
	var (
		method string
		path   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	t.Setenv(judgeURLEnv, server.URL)

	if err := WakeJudge(context.Background(), 42); err != nil {
		t.Fatalf("WakeJudge returned error: %v", err)
	}
	if method != http.MethodPost || path != "/v1/runs/42" {
		t.Fatalf("unexpected request %s %s", method, path)
	}
}

func TestWakeJudgeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv(judgeURLEnv, server.URL)

	if err := WakeJudge(context.Background(), 1); err == nil {
		t.Fatal("expected an error for a 500 wake response")
	}
}

func TestJudgeRequestAddsToken(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(judgeURLEnv, server.URL)
	t.Setenv(judgeTokenEnv, "s3cret")

	if err := JudgeReady(context.Background()); err != nil {
		t.Fatalf("JudgeReady returned error: %v", err)
	}
	if authorization != "Bearer s3cret" {
		t.Fatalf("expected bearer token, got %q", authorization)
	}
}

func TestOpenJudgeEvents(t *testing.T) {
	t.Run("streams frames", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/runs/7/events" {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			if got := r.URL.Query().Get("from"); got != "5" {
				t.Errorf("expected from=5, got %q", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "event: status\nid: 5\ndata: {\"type\":\"status\"}\n\n")
		}))
		defer server.Close()
		t.Setenv(judgeURLEnv, server.URL)

		resp, err := openJudgeEvents(context.Background(), 7, 5)
		if err != nil {
			t.Fatalf("openJudgeEvents returned error: %v", err)
		}
		defer resp.Body.Close()

		var frames []SSEFrame
		if err := ParseSSE(resp.Body, func(frame SSEFrame) error {
			frames = append(frames, frame)
			return nil
		}); err != nil {
			t.Fatalf("ParseSSE returned error: %v", err)
		}
		if len(frames) != 1 || frames[0].Event != "status" {
			t.Fatalf("unexpected frames: %+v", frames)
		}
	})

	t.Run("error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		t.Setenv(judgeURLEnv, server.URL)

		if _, err := openJudgeEvents(context.Background(), 7, 0); err == nil {
			t.Fatal("expected an error for a 404 stream response")
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		t.Setenv(judgeURLEnv, "http://127.0.0.1:1")
		if _, err := openJudgeEvents(context.Background(), 7, 0); err == nil {
			t.Fatal("expected an error for an unreachable judge")
		}
	})
}

func TestNewJudgeRequestRejectsBadURL(t *testing.T) {
	t.Setenv(judgeURLEnv, "://not-a-url")
	if _, err := newJudgeRequest(context.Background(), http.MethodGet, "/readyz", nil); err == nil {
		t.Fatal("expected an error for a malformed judge URL")
	}
}

func TestParseSSEStopsOnHandlerError(t *testing.T) {
	sentinel := io.ErrUnexpectedEOF
	err := ParseSSE(strings.NewReader("event: status\ndata: a\n\nevent: status\ndata: b\n\n"),
		func(SSEFrame) error { return sentinel })
	if err != sentinel {
		t.Fatalf("expected handler error to propagate, got %v", err)
	}
}
