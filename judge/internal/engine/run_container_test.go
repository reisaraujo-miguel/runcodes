package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/language"
	"github.com/runcodes-icmc/judge/internal/model"
	"github.com/runcodes-icmc/judge/internal/podman"
)

// fakeContainer stands in for a graded container: the test decides what the log
// stream carries, when the container exits and with which status.
type fakeContainer struct {
	logs     chan string
	errs     chan error
	exitCode int32
	waitErr  error

	killed  bool
	removed bool
}

func (c *fakeContainer) Start(context.Context) error { return nil }

func (c *fakeContainer) Logs(context.Context) *podman.LogStream {
	return &podman.LogStream{Lines: c.logs, Err: c.errs}
}

func (c *fakeContainer) Wait(time.Duration) (int32, error) { return c.exitCode, c.waitErr }
func (c *fakeContainer) Kill(context.Context) error        { c.killed = true; return nil }
func (c *fakeContainer) Remove(context.Context) error      { c.removed = true; return nil }

// fakeRuntime hands out one prepared container.
type fakeRuntime struct {
	container *fakeContainer
	created   int
}

func (r *fakeRuntime) EnsureImage(context.Context, string) error { return nil }
func (r *fakeRuntime) RemoveByName(context.Context, string) error {
	return nil
}

func (r *fakeRuntime) Create(context.Context, podman.RunConfig) (RuntimeContainer, error) {
	r.created++
	return r.container, nil
}

// newTestEngine builds an Engine whose container phase runs against a fake.
func newTestEngine(t *testing.T, messages ...string) (*Engine, *fakeContainer, string) {
	t.Helper()

	logs := make(chan string, len(messages)+1)
	for _, m := range messages {
		logs <- m
	}

	container := &fakeContainer{logs: logs, errs: make(chan error, 1)}
	runtime := &fakeRuntime{container: container}

	dir := t.TempDir()
	cfg := &config.Config{
		ExecDir:            dir,
		BaseExecTimeout:    200 * time.Millisecond,
		CompilationWait:    200 * time.Millisecond,
		DefaultCaseTimeout: 100 * time.Millisecond,
		KeepWorkspaces:     true,
	}

	engine := New(cfg, nil, nil, runtime, events.New(time.Minute),
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	return engine, container, dir
}

// testWorkspace mirrors what prepareWorkspace produces, with a fixed nonce.
func testWorkspace(dir string) *workspace {
	return &workspace{
		BaseDir:     dir,
		RemoteDir:   dir,
		SourceDir:   filepath.Join(dir, "src"),
		ExpectedDir: filepath.Join(dir, "expected"),
		RunNonce:    "test-nonce",
		Language:    language.FromExtension("c"),
		Compilable:  true,
	}
}

// TestRunContainerAcceptsOnlyMilestonesWithTheNonce is the property the nonce
// exists for: the submission's own output shares the container's log stream, so a
// program that prints "run.done" itself must not be able to tell the judge the
// run is over.
func TestRunContainerAcceptsOnlyMilestonesWithTheNonce(t *testing.T) {
	milestones := []string{"compilation.start", "compilation.done", "run.start", "run.done"}

	t.Run("nonced milestones are accepted", func(t *testing.T) {
		nonced := make([]string, 0, len(milestones))
		for _, m := range milestones {
			nonced = append(nonced, m+" test-nonce")
		}

		engine, container, dir := newTestEngine(t, nonced...)
		ws := testWorkspace(dir)

		if _, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, ws, engine.logger); err != nil {
			t.Fatalf("runContainer: %v", err)
		}
		if !container.removed {
			t.Fatal("the container was not removed")
		}
	})

	t.Run("bare milestones are ignored", func(t *testing.T) {
		// A submission printing the milestones plus the real, nonced ones.
		messages := append([]string{milestones[0], milestones[1], milestones[2]}, milestones...)
		for i, m := range messages {
			if i >= 3 {
				messages[i] = m + " test-nonce"
			}
		}

		engine, _, dir := newTestEngine(t, messages...)
		if _, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, testWorkspace(dir), engine.logger); err != nil {
			t.Fatalf("runContainer: %v", err)
		}
	})

	t.Run("only bare milestones time out", func(t *testing.T) {
		engine, container, dir := newTestEngine(t, milestones...)
		ws := testWorkspace(dir)

		_, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, ws, engine.logger)
		if err == nil {
			t.Fatal("a run whose milestones carry no nonce must not be accepted")
		}
		var timedOut *timeoutError
		if !errors.As(err, &timedOut) {
			t.Fatalf("error = %v, want a timeout waiting for the milestone", err)
		}
		// The deferred cleanup must still run. The fake container reports a clean
		// exit, so it only needs removing; the kill path is covered by
		// TestRunContainerKillsAContainerThatWillNotExit.
		if !container.removed {
			t.Fatal("the container must be removed when the run fails")
		}
	})
}

// TestRunContainerRejectsAFailedHarness pins the exit-status check: a harness that
// reports the run as done and then fails has not finished collecting its outputs,
// so they must not be graded as a result.
func TestRunContainerRejectsAFailedHarness(t *testing.T) {
	milestones := []string{
		"compilation.start test-nonce",
		"compilation.done test-nonce",
		"run.start test-nonce",
		"run.done test-nonce",
	}

	engine, container, dir := newTestEngine(t, milestones...)
	container.exitCode = 137 // killed, e.g. out of memory

	_, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, testWorkspace(dir), engine.logger)
	if err == nil {
		t.Fatal("expected the harness exit status to fail the run")
	}
	if !strings.Contains(err.Error(), "137") {
		t.Fatalf("error = %v, want it to name the exit status", err)
	}
	// The classification is what the commit is reported as.
	if status := classify(err); status != model.RunServerError {
		t.Fatalf("classify(%v) = %s, want %s", err, status, model.RunServerError)
	}
}

// TestRunContainerKillsAContainerThatWillNotExit covers the case where the
// harness never exits on its own: the lane is freed by killing the container.
func TestRunContainerKillsAContainerThatWillNotExit(t *testing.T) {
	milestones := []string{
		"compilation.start test-nonce",
		"compilation.done test-nonce",
		"run.start test-nonce",
		"run.done test-nonce",
	}

	engine, container, dir := newTestEngine(t, milestones...)
	container.waitErr = context.DeadlineExceeded

	if _, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, testWorkspace(dir), engine.logger); err != nil {
		t.Fatalf("runContainer: %v", err)
	}
	if !container.killed {
		t.Fatal("a container that does not exit must be killed")
	}
	if !container.removed {
		t.Fatal("the container was not removed")
	}
}

// TestRunContainerSkipsCompilationMilestonesForInterpretedLanguages keeps the
// existing behaviour: only compilable languages print the compilation phase.
func TestRunContainerSkipsCompilationMilestonesForInterpretedLanguages(t *testing.T) {
	engine, _, dir := newTestEngine(t, "run.start test-nonce", "run.done test-nonce")

	ws := testWorkspace(dir)
	ws.Language = language.FromExtension("py")
	ws.Compilable = false

	if _, err := engine.runContainer(context.Background(), &model.Commit{ID: 1}, ws, engine.logger); err != nil {
		t.Fatalf("runContainer: %v", err)
	}
}

// TestWriteContainerConfigCarriesTheContract checks the keys the harness reads:
// the nonce, the source file and the per-case limits (only when the exercise set
// them, so an unauthored limit does not become a limit of zero).
func TestWriteContainerConfigCarriesTheContract(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{cfg: &config.Config{
		MonitorMaxFileSize: 5 * 1024 * 1024,
		MonitorMaxMemSize:  256 * 1024 * 1024,
		CompilationTimeout: 10 * time.Second,
		DefaultCaseTimeout: 3 * time.Second,
	}}

	ws := &workspace{
		BaseDir:  dir,
		RunNonce: "abc123",
		TestCases: []model.TestCase{
			{ID: 3, CPUTimeLimit: 5, MemUsageLimit: 1 << 20, StackLimit: 1 << 19},
			{ID: 4, CPUTimeLimit: 0},
		},
	}

	if err := e.writeContainerConfig(&model.Commit{ID: 1, S3Key: "uuid/main.c"}, ws); err != nil {
		t.Fatalf("writeContainerConfig: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "container.config"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, want := range []string{
		"run_nonce='abc123'",
		"src_file='main.c'",
		"t_3=5",
		"ms_3=1048576",
		"stack_3=524288",
		"t_4=3", // falls back to the default case timeout
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in container.config:\n%s", want, got)
		}
	}

	// Case 4 authored no limits: its keys must not appear at all.
	for _, unwanted := range []string{"ms_4=", "fs_4=", "stack_4=", "fs_3="} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in container.config:\n%s", unwanted, got)
		}
	}
}

// TestWriteContainerConfigRejectsAnUnsafeFileName pins the input that reaches a
// file the harness sources with bash: a line break would end the assignment and
// turn the rest of the name into an instruction for the container's root.
func TestWriteContainerConfigRejectsAnUnsafeFileName(t *testing.T) {
	dir := t.TempDir()
	e := &Engine{cfg: &config.Config{DefaultCaseTimeout: time.Second}}

	ws := &workspace{BaseDir: dir, RunNonce: "abc123"}
	// Filename() is the part after the last slash, so the break has to be inside
	// the base name to reach the config file.
	commit := &model.Commit{ID: 1, S3Key: "uuid/main.c\nrm -rf ."}

	if err := e.writeContainerConfig(commit, ws); err == nil {
		t.Fatal("a file name with a line break must be rejected")
	}

	if _, err := os.Stat(filepath.Join(dir, "container.config")); err == nil {
		t.Fatal("no config file must be written for an invalid name")
	}
}

func TestNewRunNonceIsRandomAndHex(t *testing.T) {
	first, err := newRunNonce()
	if err != nil {
		t.Fatalf("newRunNonce: %v", err)
	}
	second, err := newRunNonce()
	if err != nil {
		t.Fatalf("newRunNonce: %v", err)
	}

	if len(first) != 32 {
		t.Fatalf("nonce %q has length %d, want 32 hex characters", first, len(first))
	}
	if first == second {
		t.Fatal("two runs got the same nonce")
	}
}
