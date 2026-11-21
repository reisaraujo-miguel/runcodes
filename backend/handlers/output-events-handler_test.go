package handlers

import (
	"net/http/httptest"
	"strings"
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

func TestSSEFrameState(t *testing.T) {
	frame := []byte("event: artifact\nid: 4\ndata: {\"type\":\"artifact\",\"state\":\"ready\"}\n\n")
	if got := sseFrameState(frame); got != "ready" {
		t.Fatalf("sseFrameState = %q, want ready", got)
	}
	if got := sseFrameState([]byte("event: artifact\nid: 4\ndata: {}\n\n")); got != "" {
		t.Fatalf("sseFrameState without a state = %q, want empty", got)
	}
	if got := sseFrameState([]byte("event: artifact\n\n")); got != "" {
		t.Fatalf("sseFrameState without data = %q, want empty", got)
	}
}

func TestOutputFilesFrameAddsTheRunLevelEntry(t *testing.T) {
	files := []OutputFileEntry{{Path: "test_v2_1/a.txt", Size: 4}}
	frame := outputFilesFrame(files, nil)
	if len(frame.Artifacts) != 1 || frame.Artifacts[0].Kind != "output" ||
		frame.Artifacts[0].TestCaseID != nil {
		t.Fatalf("artifacts = %+v, want the run-level entry", frame.Artifacts)
	}

	// No files: the placeholder must not claim a download exists.
	if got := outputFilesFrame(nil, nil); len(got.Artifacts) != 0 {
		t.Fatalf("artifacts = %+v, want none", got.Artifacts)
	}

	caseID := int64(7)
	existing := []models.SubmissionArtifact{{Kind: "output", TestCaseID: &caseID, State: "failed"}}
	if got := outputFilesFrame(files, existing); len(got.Artifacts) != 2 {
		t.Fatalf("artifacts = %+v, want the run-level entry added once", got.Artifacts)
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

func TestSubmissionDownloadFilename(t *testing.T) {
	tests := []struct {
		name   string
		title  string
		number int64
		want   string
	}{
		{"plain title", "Soma de vetores", 3, "Soma de vetores_3.zip"},
		{"path separators", "a/b\\c", 1, "a_b_c_1.zip"},
		{"quotes", `say "hi"`, 2, "say _hi__2.zip"},
		{"control characters", "line\nbreak", 4, "linebreak_4.zip"},
		{"empty title", "   ", 5, "submission_5.zip"},
		{"long title", strings.Repeat("a", 200), 6, strings.Repeat("a", 80) + "_6.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := submissionDownloadFilename(tt.title, tt.number); got != tt.want {
				t.Fatalf("submissionDownloadFilename(%q, %d) = %q, want %q",
					tt.title, tt.number, got, tt.want)
			}
		})
	}
}
