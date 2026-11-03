package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/model"
)

// fakeOutputStore records what the engine published, standing in for the S3
// client so the upload path is exercised without object storage. Background
// workers call it concurrently, so the recorded state is guarded.
type fakeOutputStore struct {
	mu       sync.Mutex
	uploaded map[string]string
	err      error
}

func (f *fakeOutputStore) UploadGeneratedFile(
	_ context.Context, _ int64, rel, src string,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return 0, f.err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}

	if f.uploaded == nil {
		f.uploaded = make(map[string]string)
	}
	f.uploaded[rel] = string(data)

	return int64(len(data)), nil
}

// newOutputEngine builds the engine state the publishing helpers need.
func newOutputEngine(t *testing.T) (*Engine, *fakeOutputStore, *workspace) {
	t.Helper()

	store := &fakeOutputStore{}
	return &Engine{
		cfg: &config.Config{
			ExecDir:          t.TempDir(),
			MaxArtifactBytes: 1 << 20,
		},
		outputs: store,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, store, &workspace{BaseDir: t.TempDir()}
}

// publishTest runs one case's decision and upload synchronously, the way the run's
// background publisher does: the request is built on the log-stream goroutine and
// the transfer happens after it.
func publishTest(
	t *testing.T, engine *Engine, commitID int64, testCase model.TestCase,
	result model.CaseResult, ws *workspace,
) (int, error) {
	t.Helper()

	req, ok, err := engine.casePublishRequest(testCase, result, ws)
	if err != nil || !ok {
		return 0, err
	}

	uploaded, _, err := engine.publishGeneratedFiles(context.Background(), commitID, req)
	return uploaded, err
}

// writeWorkspaceFile creates a file under the workspace, making its parents.
func writeWorkspaceFile(t *testing.T, baseDir, rel, data string) {
	t.Helper()

	path := filepath.Join(baseDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", rel, err)
	}
}

func TestIsInternalOutputFile(t *testing.T) {
	tests := []struct {
		name     string
		internal bool
	}{
		{"1.output", true},
		{"12.error", true},
		{"3.monitor_out", true},
		{"4.monitor_err", true},
		{"compilation.out", true},
		{"compilation.err", true},
		{"compilation.monitor_out", true},
		{"compilation.monitor_err", true},
		{"nested/1.output", true},
		{"result.output", false},
		{"notes.error", false},
		{"compilation_notes.txt", false},
		{"1.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInternalOutputFile(tt.name); got != tt.internal {
				t.Fatalf("isInternalOutputFile(%q) = %v, want %v", tt.name, got, tt.internal)
			}
		})
	}
}

func TestPublishCaseOutputs(t *testing.T) {
	engine, store, ws := newOutputEngine(t)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_11/hello.txt", "hello")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_11/nested/result.txt", "nested")

	testCase := model.TestCase{ID: 11, FileSizeLimit: 256 << 20}
	result := model.CaseResult{Status: model.CaseCorrect}
	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 2 {
		t.Fatalf("uploaded = %d, want 2", uploaded)
	}

	want := map[string]string{
		"test_11/hello.txt":         "hello",
		"test_11/nested/result.txt": "nested",
	}
	if len(store.uploaded) != len(want) {
		t.Fatalf("uploaded = %v, want %v", store.uploaded, want)
	}
	for rel, content := range want {
		if got := store.uploaded[rel]; got != content {
			t.Fatalf("%s = %q, want %q", rel, got, content)
		}
	}

	// The local copy is gone, so the workspace is bounded by the current case.
	if _, err := os.Stat(filepath.Join(ws.BaseDir, "outputfiles", "case_11")); !os.IsNotExist(err) {
		t.Fatalf("expected the collection directory to be removed, stat error: %v", err)
	}
}

func TestPublishRootOutputs(t *testing.T) {
	engine, store, ws := newOutputEngine(t)

	// A file the submission wrote straight into the output directory, next to
	// the runner's own reports and a per-case directory already published.
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/explicit.txt", "user")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/1.output", "internal")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/2.monitor_out", "internal")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/compilation.err", "internal")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_3/produced.txt", "case output")

	uploaded, err := engine.publishRootOutputs(context.Background(), 9, ws)
	if err != nil {
		t.Fatalf("publishRootOutputs: %v", err)
	}
	if uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", uploaded)
	}

	want := map[string]string{"outputfiles/explicit.txt": "user"}
	if len(store.uploaded) != len(want) || store.uploaded["outputfiles/explicit.txt"] != "user" {
		t.Fatalf("uploaded = %v, want %v", store.uploaded, want)
	}
}

func TestPublishGeneratedFilesSkipsSymlinks(t *testing.T) {
	engine, store, ws := newOutputEngine(t)

	// A file outside the collection directory that a planted symlink points at;
	// publishing it would leak whatever the judge can read.
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dir := filepath.Join(ws.BaseDir, "outputfiles", "case_4")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_4/real.txt", "real")

	testCase := model.TestCase{ID: 4, FileSizeLimit: 64}
	result := model.CaseResult{Status: model.CaseCorrect}
	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", uploaded)
	}
	if _, ok := store.uploaded["test_4/link.txt"]; ok {
		t.Fatal("a symlink must not be published")
	}
	if got := store.uploaded["test_4/real.txt"]; got != "real" {
		t.Fatalf("real.txt = %q, want real", got)
	}
}

func TestPublishGeneratedFilesRespectsTheCaseLimit(t *testing.T) {
	engine, store, ws := newOutputEngine(t)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_5/fits.bin", "1234")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_5/too_big.bin", "12345")

	testCase := model.TestCase{ID: 5, FileSizeLimit: 4}
	result := model.CaseResult{Status: model.CaseCorrect}
	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", uploaded)
	}
	if _, ok := store.uploaded["test_5/fits.bin"]; !ok {
		t.Fatal("a file exactly at the case limit must be published")
	}
	if _, ok := store.uploaded["test_5/too_big.bin"]; ok {
		t.Fatal("a file past the case limit must not be published")
	}
}

func TestPublishGeneratedFilesWithoutALimitUsesTheCeiling(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.cfg.MaxArtifactBytes = 4

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_6/fits.bin", "1234")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_6/too_big.bin", "12345")

	testCase := model.TestCase{ID: 6}
	result := model.CaseResult{Status: model.CaseCorrect}
	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", uploaded)
	}
	if _, ok := store.uploaded["test_6/too_big.bin"]; ok {
		t.Fatal("a case without a limit must still respect the platform ceiling")
	}
}

// TestPublishCaseOutputsDoesNotShareALimitAcrossCases pins that a large file in
// one case does not make the following cases unpublishable. A per-run byte budget
// did exactly that: the first case's file consumed it and every later case's
// download silently went missing.
func TestPublishCaseOutputsDoesNotShareALimitAcrossCases(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.cfg.MaxArtifactBytes = 1024

	// Case 1 hits its own small limit and publishes a file of that size.
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_1/first.bin", "1234567890")
	// Case 2 has a larger limit and writes more than case 1 did.
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_2/second.bin", "12345678901234567890")

	first := model.TestCase{ID: 1, FileSizeLimit: 10}
	if uploaded, err := publishTest(t, engine, 7, first, model.CaseResult{Status: model.CaseBadFormat}, ws); err != nil || uploaded != 1 {
		t.Fatalf("case 1: uploaded = %d, err = %v; want 1, nil", uploaded, err)
	}

	second := model.TestCase{ID: 2, FileSizeLimit: 20}
	if uploaded, err := publishTest(t, engine, 7, second, model.CaseResult{Status: model.CaseCorrect}, ws); err != nil || uploaded != 1 {
		t.Fatalf("case 2: uploaded = %d, err = %v; want 1, nil", uploaded, err)
	}

	if _, ok := store.uploaded["test_1/first.bin"]; !ok {
		t.Fatal("case 1's file was not published")
	}
	if _, ok := store.uploaded["test_2/second.bin"]; !ok {
		t.Fatal("case 2's file was not published after case 1 used more bytes")
	}
}

func TestPublishGeneratedFilesSurvivesUploadFailure(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	store.err = errors.New("bucket unavailable")

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_6/a.txt", "a")

	testCase := model.TestCase{ID: 6, FileSizeLimit: 64}
	result := model.CaseResult{Status: model.CaseCorrect}
	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 0 {
		t.Fatalf("uploaded = %d, want 0", uploaded)
	}

	// The directory is still removed: a failed upload must not leave the files
	// behind to accumulate for the rest of the suite.
	if _, err := os.Stat(filepath.Join(ws.BaseDir, "outputfiles", "case_6")); !os.IsNotExist(err) {
		t.Fatalf("expected the collection directory to be removed, stat error: %v", err)
	}
}

// TestPublishCaseOutputsReportsAMissingCollection pins the upgrade signal: an
// image from before the collection step leaves no `case_<id>` directory, and the
// run reports it instead of quietly publishing nothing.
func TestPublishCaseOutputsReportsAMissingCollection(t *testing.T) {
	engine, _, ws := newOutputEngine(t)

	testCase := model.TestCase{ID: 3, FileSizeLimit: 1024}
	_, err := publishTest(t, engine, 7, testCase, model.CaseResult{Status: model.CaseCorrect}, ws)
	if !errors.Is(err, errNoCollectionDir) {
		t.Fatalf("err = %v, want errNoCollectionDir", err)
	}
}

// TestPublishCaseOutputsAcceptsAnEmptyCollection pins the other side of the
// signal: a case that wrote nothing still has a directory, and that must not be
// reported as an outdated image.
func TestPublishCaseOutputsAcceptsAnEmptyCollection(t *testing.T) {
	engine, _, ws := newOutputEngine(t)

	if err := os.MkdirAll(filepath.Join(ws.BaseDir, "outputfiles", "case_3"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	testCase := model.TestCase{ID: 3, FileSizeLimit: 1024}
	uploaded, err := publishTest(t, engine, 7, testCase, model.CaseResult{Status: model.CaseCorrect}, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 0 {
		t.Fatalf("uploaded = %d, want 0", uploaded)
	}
}

// TestPublishCaseOutputsSkipsAnInterruptedCase pins that a case killed by a limit
// or a signal publishes nothing: its file was still being written when the process
// died, so the collected bytes are a truncated file, not a result.
func TestPublishCaseOutputsSkipsAnInterruptedCase(t *testing.T) {
	engine, store, ws := newOutputEngine(t)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_2/partial.bin", "12345")

	testCase := model.TestCase{ID: 2, FileSizeLimit: 5}
	result := model.CaseResult{
		TestCaseID: 2,
		Status:     model.CaseKilledBySignal,
		Reason:     model.CaseReasonFileSizeLimit,
	}

	uploaded, err := publishTest(t, engine, 7, testCase, result, ws)
	if err != nil {
		t.Fatalf("publishCaseOutputs: %v", err)
	}
	if uploaded != 0 {
		t.Fatalf("uploaded = %d, want 0", uploaded)
	}
	if len(store.uploaded) != 0 {
		t.Fatalf("uploaded = %v, want nothing", store.uploaded)
	}
	if _, err := os.Stat(filepath.Join(ws.BaseDir, "outputfiles", "case_2")); !os.IsNotExist(err) {
		t.Fatalf("expected the interrupted case's files to be removed, stat error: %v", err)
	}
}

// blockingOutputStore holds every upload until release is closed, so a test can
// fill the publisher's workers and queue deterministically.
type blockingOutputStore struct {
	release chan struct{}
	active  atomic.Int32
}

func (b *blockingOutputStore) UploadGeneratedFile(
	_ context.Context, _ int64, _, _ string,
) (int64, error) {
	b.active.Add(1)
	defer b.active.Add(-1)

	<-b.release

	return 0, nil
}

// waitFor polls until cond holds, failing the test after a short deadline.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatal("condition was not met before the deadline")
}

// artifactEvents drains the subscription and returns the artifact frames in order.
func artifactEvents(sub *events.Subscription) []map[string]any {
	var frames []map[string]any
	for {
		select {
		case frame, ok := <-sub.Ch:
			if !ok {
				return frames
			}
			if frame.Name != events.NameArtifact {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal(frame.Data, &payload); err == nil {
				frames = append(frames, payload)
			}
		default:
			return frames
		}
	}
}

// TestQueueCaseOutputsAnnouncesPendingThenReady pins the two events the results
// panel relies on: a placeholder while the file uploads, then the link.
func TestQueueCaseOutputsAnnouncesPendingThenReady(t *testing.T) {
	engine, _, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_5/a.txt", "alpha")

	sub := engine.hub.Subscribe(7, 0)
	defer sub.Cancel()

	publisher := newOutputPublisher(context.Background(), engine, 7)
	if err := engine.queueCaseOutputs(
		publisher, 7,
		model.TestCase{ID: 5, FileSizeLimit: 64, ShowUserOutput: true},
		model.CaseResult{Status: model.CaseCorrect},
		ws,
	); err != nil {
		t.Fatalf("queueCaseOutputs: %v", err)
	}
	publisher.Close()

	frames := artifactEvents(sub)
	if len(frames) != 2 {
		t.Fatalf("artifact events = %d, want 2: %v", len(frames), frames)
	}
	for i, want := range []string{events.ArtifactPending, events.ArtifactReady} {
		if got := frames[i]["state"]; got != want {
			t.Fatalf("event %d state = %v, want %s", i, got, want)
		}
		if got := frames[i]["test_case_id"]; got != float64(5) {
			t.Fatalf("event %d test_case_id = %v, want 5", i, got)
		}
	}
}

// TestQueueCaseOutputsAnnouncesFailure pins that a case whose upload fails clears
// its placeholder instead of leaving it spinning.
func TestQueueCaseOutputsAnnouncesFailure(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)
	store.err = errors.New("bucket unavailable")

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_6/a.txt", "alpha")

	sub := engine.hub.Subscribe(7, 0)
	defer sub.Cancel()

	publisher := newOutputPublisher(context.Background(), engine, 7)
	if err := engine.queueCaseOutputs(
		publisher, 7,
		model.TestCase{ID: 6, FileSizeLimit: 64, ShowUserOutput: true},
		model.CaseResult{Status: model.CaseCorrect},
		ws,
	); err != nil {
		t.Fatalf("queueCaseOutputs: %v", err)
	}
	publisher.Close()

	frames := artifactEvents(sub)
	if len(frames) != 2 {
		t.Fatalf("artifact events = %d, want 2: %v", len(frames), frames)
	}
	if got := frames[0]["state"]; got != events.ArtifactPending {
		t.Fatalf("first state = %v, want pending", got)
	}
	if got := frames[1]["state"]; got != events.ArtifactFailed {
		t.Fatalf("second state = %v, want failed", got)
	}
}

// TestQueueCaseOutputsIgnoresACaseThatWroteNothing pins that the harness's
// unconditional collection directory does not produce an upload placeholder for a
// case that generated no files.
func TestQueueCaseOutputsIgnoresACaseThatWroteNothing(t *testing.T) {
	engine, _, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)

	if err := os.MkdirAll(
		filepath.Join(ws.BaseDir, "outputfiles", "case_4"), 0o755,
	); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	sub := engine.hub.Subscribe(7, 0)
	defer sub.Cancel()

	publisher := newOutputPublisher(context.Background(), engine, 7)
	if err := engine.queueCaseOutputs(
		publisher, 7,
		model.TestCase{ID: 4, FileSizeLimit: 64, ShowUserOutput: true},
		model.CaseResult{Status: model.CaseCorrect},
		ws,
	); err != nil {
		t.Fatalf("queueCaseOutputs: %v", err)
	}
	publisher.Close()

	if frames := artifactEvents(sub); len(frames) != 0 {
		t.Fatalf("artifact events = %v, want none for a case that wrote nothing", frames)
	}
}

// TestQueueCaseOutputsSkipsAHiddenCase pins that a case whose output the author
// did not release is neither uploaded nor announced: the client must not show an
// upload placeholder for a download it will never get.
func TestQueueCaseOutputsSkipsAHiddenCase(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_9/a.txt", "alpha")

	sub := engine.hub.Subscribe(7, 0)
	defer sub.Cancel()

	publisher := newOutputPublisher(context.Background(), engine, 7)
	if err := engine.queueCaseOutputs(
		publisher, 7,
		model.TestCase{ID: 9, FileSizeLimit: 64, ShowUserOutput: false},
		model.CaseResult{Status: model.CaseCorrect},
		ws,
	); err != nil {
		t.Fatalf("queueCaseOutputs: %v", err)
	}
	publisher.Close()

	if frames := artifactEvents(sub); len(frames) != 0 {
		t.Fatalf("artifact events = %v, want none for a hidden case", frames)
	}
	if len(store.uploaded) != 0 {
		t.Fatalf("uploaded = %v, want nothing", store.uploaded)
	}
	if _, err := os.Stat(filepath.Join(ws.BaseDir, "outputfiles", "case_9")); !os.IsNotExist(err) {
		t.Fatalf("the hidden case's files were kept, stat error: %v", err)
	}
}

// TestQueueCaseOutputsStagesOutsideTheWorkspace pins that queued files leave the
// run's workspace, so the upload backlog never counts against the workspace quota.
func TestQueueCaseOutputsStagesOutsideTheWorkspace(t *testing.T) {
	engine, _, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)
	store := &blockingOutputStore{release: make(chan struct{})}
	engine.outputs = store

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_3/a.txt", "alpha")

	publisher := newOutputPublisher(context.Background(), engine, 7)
	if err := engine.queueCaseOutputs(
		publisher, 7,
		model.TestCase{ID: 3, FileSizeLimit: 64, ShowUserOutput: true},
		model.CaseResult{Status: model.CaseCorrect},
		ws,
	); err != nil {
		t.Fatalf("queueCaseOutputs: %v", err)
	}

	pending := engine.pendingOutputDir(7, 3)
	waitFor(t, func() bool {
		_, err := os.Stat(filepath.Join(pending, "a.txt"))
		return err == nil
	})
	if _, err := os.Stat(filepath.Join(ws.BaseDir, "outputfiles", "case_3")); !os.IsNotExist(err) {
		t.Fatalf("the workspace kept the queued files, stat error: %v", err)
	}

	close(store.release)
	publisher.Close()

	if _, err := os.Stat(pending); !os.IsNotExist(err) {
		t.Fatalf("the pending area survived the run, stat error: %v", err)
	}
}

// TestOutputPublisherRefusesWhenFull pins that a stalled upload never blocks the
// goroutine reading the container's milestones: once the workers and the queue are
// busy, the next case is refused immediately.
func TestOutputPublisherRefusesWhenFull(t *testing.T) {
	engine, _, ws := newOutputEngine(t)
	store := &blockingOutputStore{release: make(chan struct{})}
	engine.outputs = store
	engine.hub = events.New(time.Minute)

	capacity := outputPublishWorkers + outputPublishQueue
	for id := int64(1); id <= int64(capacity)+1; id++ {
		writeWorkspaceFile(
			t, ws.BaseDir,
			fmt.Sprintf("outputfiles/case_%d/file.txt", id), "x",
		)
	}

	publisher := newOutputPublisher(context.Background(), engine, 7)

	submit := func(id int64) bool {
		t.Helper()
		req, ok, err := engine.casePublishRequest(
			model.TestCase{ID: id, FileSizeLimit: 64},
			model.CaseResult{Status: model.CaseCorrect}, ws,
		)
		if err != nil || !ok {
			t.Fatalf("case %d: request ok=%v err=%v", id, ok, err)
		}

		return publisher.Submit(req)
	}

	// The workers take tasks as they arrive, so the loop fills the queue while
	// they are blocked in the store.
	accepted := 0
	for id := int64(1); id <= int64(capacity); id++ {
		if submit(id) {
			accepted++
		}
	}
	waitFor(t, func() bool { return store.active.Load() == outputPublishWorkers })
	for accepted < capacity {
		if submit(int64(accepted) + 1) {
			accepted++
		}
	}

	// Everything is busy: the caller is told instead of waiting.
	start := time.Now()
	if submit(int64(capacity) + 1) {
		t.Fatal("a submission with a full queue must be refused")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("refusing took %s; the log stream must not wait", elapsed)
	}

	close(store.release)
	publisher.Close()
}

// TestOutputPublisherStopsAfterRepeatedFailures pins the circuit breaker: a store
// that keeps failing must stop costing the run an upload per case.
func TestOutputPublisherStopsAfterRepeatedFailures(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)
	store.err = errors.New("bucket unavailable")

	for id := int64(1); id <= outputPublishFailureLimit+1; id++ {
		writeWorkspaceFile(
			t, ws.BaseDir,
			fmt.Sprintf("outputfiles/case_%d/file.txt", id), "x",
		)
	}

	publisher := newOutputPublisher(context.Background(), engine, 7)

	submit := func(id int64) bool {
		t.Helper()
		req, ok, err := engine.casePublishRequest(
			model.TestCase{ID: id, FileSizeLimit: 64},
			model.CaseResult{Status: model.CaseCorrect}, ws,
		)
		if err != nil || !ok {
			t.Fatalf("case %d: request ok=%v err=%v", id, ok, err)
		}

		return publisher.Submit(req)
	}

	// Feed cases until the failures trip the breaker. Uploads fail immediately in
	// the fake store, so this loop does not wait on the bounded queue.
	for id := int64(1); id <= int64(outputPublishFailureLimit); id++ {
		submit(id)
	}
	waitFor(t, publisher.Degraded)

	if submit(int64(outputPublishFailureLimit) + 1) {
		t.Fatal("a degraded publisher must refuse more work")
	}

	publisher.Close()
}

func TestPublishGeneratedFilesWithoutStorage(t *testing.T) {
	engine, _, ws := newOutputEngine(t)
	engine.outputs = nil

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_8/a.txt", "a")

	testCase := model.TestCase{ID: 8, FileSizeLimit: 64}
	_, err := publishTest(t, engine, 7, testCase, model.CaseResult{Status: model.CaseCorrect}, ws)
	if err == nil || errors.Is(err, errNoCollectionDir) {
		t.Fatalf("err = %v, want a storage error", err)
	}
}

func TestCollectGeneratedFilesMissingDir(t *testing.T) {
	files, err := collectGeneratedFiles(filepath.Join(t.TempDir(), "absent"), nil)
	if err != nil {
		t.Fatalf("collectGeneratedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files = %v, want none", files)
	}
}

// TestOutputPublisherUploadsInTheBackground pins the behaviour the results panel
// depends on: files are uploaded, the artifact event is published once, and Close
// waits for everything queued.
func TestOutputPublisherUploadsInTheBackground(t *testing.T) {
	engine, store, ws := newOutputEngine(t)
	engine.hub = events.New(time.Minute)

	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_1/a.txt", "alpha")
	writeWorkspaceFile(t, ws.BaseDir, "outputfiles/case_2/b.txt", "beta")

	sub := engine.hub.Subscribe(7, 0)
	defer sub.Cancel()

	publisher := newOutputPublisher(context.Background(), engine, 7)
	for _, id := range []int64{1, 2} {
		req, ok, err := engine.casePublishRequest(
			model.TestCase{ID: id, FileSizeLimit: 64},
			model.CaseResult{Status: model.CaseCorrect}, ws,
		)
		if err != nil || !ok {
			t.Fatalf("case %d: request ok=%v err=%v", id, ok, err)
		}
		if !publisher.Submit(req) {
			t.Fatalf("case %d: submit was refused", id)
		}
	}

	if files := publisher.Close(); files != 2 {
		t.Fatalf("published files = %d, want 2", files)
	}
	if len(store.uploaded) != 2 {
		t.Fatalf("uploaded = %v, want two files", store.uploaded)
	}

	artifacts := 0
	for {
		select {
		case frame, ok := <-sub.Ch:
			if !ok {
				goto drained
			}
			if frame.Name == events.NameArtifact {
				artifacts++
			}
		default:
			goto drained
		}
	}
drained:
	// One per upload: each tells the client that new files are downloadable.
	if artifacts != 2 {
		t.Fatalf("artifact events = %d, want one per upload (2)", artifacts)
	}

	// Submitting after Close must be harmless: the run ended and the workspace is
	// about to be removed.
	if publisher.Submit(publishRequest{Dir: filepath.Join(ws.BaseDir, "absent")}) {
		t.Fatal("Submit accepted work after Close")
	}
}
