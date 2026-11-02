package engine

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/language"
	"github.com/runcodes-icmc/judge/internal/model"
	"github.com/runcodes-icmc/judge/internal/podman"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// A container that missed its startup budget is an infrastructure failure, not
// the submission's timeout: nothing the student wrote has run yet.
func TestAwaitStartupConvertsATimeout(t *testing.T) {
	const budget = 20 * time.Millisecond

	e := &Engine{
		cfg:    &config.Config{StartupWait: budget},
		logger: discardLogger(),
	}
	stream := &podman.LogStream{Lines: make(chan string)}

	err := e.awaitStartup(stream, "run.start", "nonce")

	var startup *startupError
	if !errors.As(err, &startup) {
		t.Fatalf("awaitStartup error = %v, want a *startupError", err)
	}
	if startup.Milestone != "run.start" || startup.Wait != budget {
		t.Fatalf("startup error = %+v", startup)
	}
	if got := classify(err); got != model.RunServerError {
		t.Fatalf("classify = %s, want server_error", got)
	}
}

func TestAwaitStartupAcceptsTheFirstMilestone(t *testing.T) {
	logs := make(chan string, 1)
	logs <- "run.start nonce"

	e := &Engine{
		cfg:    &config.Config{StartupWait: time.Second},
		logger: discardLogger(),
	}

	if err := e.awaitStartup(
		&podman.LogStream{Lines: logs}, "run.start", "nonce",
	); err != nil {
		t.Fatalf("awaitStartup: %v", err)
	}
}

// The alert the administrators receive names the run, the language and how long
// the judge waited; it is the only record of a failure the student cannot fix.
func TestAlertStartupFailurePublishesTheAlert(t *testing.T) {
	hub := events.New(time.Minute)
	sub := hub.Subscribe(6, 0)
	defer sub.Cancel()

	cfg := &config.Config{
		ImageFormat: "registry.example.com/runner-%s:test",
		StartupWait: 30 * time.Second,
	}
	e := New(cfg, nil, nil, nil, hub, discardLogger())

	ws := &workspace{
		Language:  language.FromExtension("py"),
		TestCases: []model.TestCase{{ID: 2}, {ID: 3}},
	}
	commit := &model.Commit{ID: 6, S3Key: "commits/main.py"}

	e.alertStartupFailure(commit, ws,
		&startupError{Milestone: "run.start", Wait: cfg.StartupWait},
		discardLogger(),
	)

	var frame events.Frame
	select {
	case frame = <-sub.Ch:
	case <-time.After(time.Second):
		t.Fatal("no alert frame published")
	}

	if frame.Name != events.NameAlert {
		t.Fatalf("frame name = %q, want alert", frame.Name)
	}

	var payload struct {
		Type     string         `json:"type"`
		CommitID int64          `json:"commit_id"`
		Severity string         `json:"severity"`
		Code     string         `json:"code"`
		Message  string         `json:"message"`
		Details  map[string]any `json:"details"`
	}
	if err := json.Unmarshal(frame.Data, &payload); err != nil {
		t.Fatalf("unmarshal alert: %v", err)
	}

	if payload.CommitID != 6 || payload.Type != events.NameAlert {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Severity != model.AlertSeverityCritical || payload.Code != model.AlertStartupTimeout {
		t.Fatalf("severity/code = %s/%s", payload.Severity, payload.Code)
	}
	if payload.Message == "" {
		t.Fatal("the alert carries no message")
	}
	if got := payload.Details["image"]; got != "registry.example.com/runner-python:test" {
		t.Fatalf("image = %v", got)
	}
	if got := payload.Details["language"]; got != "Python" {
		t.Fatalf("language = %v", got)
	}
	if got := payload.Details["milestone"]; got != "run.start" {
		t.Fatalf("milestone = %v", got)
	}
	if got := payload.Details["wait_seconds"]; got != float64(30) {
		t.Fatalf("wait_seconds = %v", got)
	}
	if got := payload.Details["test_cases"]; got != float64(2) {
		t.Fatalf("test_cases = %v", got)
	}
}
