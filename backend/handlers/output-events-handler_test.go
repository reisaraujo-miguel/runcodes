package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/models"

	"golang.org/x/net/websocket"
)

func TestSSEFrameEvent(t *testing.T) {
	frame := []byte("event: artifact\nid: 4\ndata: {\"type\":\"artifact\"}\n\n")
	if got := sseFrameEvent(frame); got != "artifact" {
		t.Fatalf("sseFrameEvent = %q, want artifact", got)
	}
	if got := sseFrameEvent([]byte("data: {}\n\n")); got != "" {
		t.Fatalf("sseFrameEvent without an event = %q, want empty", got)
	}
}

func TestHasRunLevelArtifact(t *testing.T) {
	caseID := int64(7)
	if hasRunLevelArtifact([]models.SubmissionArtifact{{Kind: "output", TestCaseID: &caseID}}) {
		t.Fatal("a per-case artifact is not the run-level entry")
	}
	if !hasRunLevelArtifact([]models.SubmissionArtifact{{Kind: "output"}}) {
		t.Fatal("missing run-level entry")
	}
}

func TestOutputSocketOriginAllowed(t *testing.T) {
	previous := config.C
	config.C = &config.Config{FrontendOrigin: "http://localhost:5173"}
	t.Cleanup(func() { config.C = previous })

	wsConfig := &websocket.Config{Version: websocket.ProtocolVersionHybi13}

	t.Run("accepts the page's own origin", func(t *testing.T) {
		request := httptest.NewRequest("GET", "http://localhost:8080/api/v1/submissions/1/output-events", nil)
		request.Host = "localhost:8080"
		request.Header.Set("Origin", "http://localhost:8080")

		if err := outputSocketOriginAllowed(wsConfig, request); err != nil {
			t.Fatalf("same-origin handshake refused: %v", err)
		}
	})

	t.Run("accepts the configured SPA origin", func(t *testing.T) {
		request := httptest.NewRequest("GET", "http://localhost:8443/api/v1/submissions/1/output-events", nil)
		request.Host = "localhost:8443"
		request.Header.Set("Origin", "http://localhost:5173")

		if err := outputSocketOriginAllowed(wsConfig, request); err != nil {
			t.Fatalf("configured origin refused: %v", err)
		}
	})

	t.Run("refuses another site", func(t *testing.T) {
		request := httptest.NewRequest("GET", "http://localhost:8443/api/v1/submissions/1/output-events", nil)
		request.Host = "localhost:8443"
		request.Header.Set("Origin", "https://evil.example")

		if err := outputSocketOriginAllowed(wsConfig, request); err == nil {
			t.Fatal("a foreign origin was accepted")
		}
	})
}
