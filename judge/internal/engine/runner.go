// Package engine runs a claimed commit end to end: prepare the workspace, drive
// the container through compilation and test execution, grade the outputs and
// publish the resulting events. It performs no persistence.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/model"
	"github.com/runcodes-icmc/judge/internal/podman"
	"github.com/runcodes-icmc/judge/internal/storage"
	"github.com/runcodes-icmc/judge/internal/store"
	"github.com/runcodes-icmc/judge/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ContainerRuntime is the slice of podman the engine drives. It exists so the run
// phase can be exercised without a container engine.
type ContainerRuntime interface {
	// EnsureImage makes the image available locally. It reports whether this
	// call had to pull it, so the run can distinguish a cold image from a warm
	// one in its phase timings.
	EnsureImage(ctx context.Context, image string) (pulled bool, err error)
	RemoveByName(ctx context.Context, name string) error
	Create(ctx context.Context, rc podman.RunConfig) (RuntimeContainer, error)
}

// RuntimeContainer is a created container.
type RuntimeContainer interface {
	Start(ctx context.Context) error
	Logs(ctx context.Context) *podman.LogStream
	Wait(timeout time.Duration) (int32, error)
	Kill(ctx context.Context) error
	Remove(ctx context.Context) error
}

// PodmanRuntime adapts a podman client to the interface the engine drives. The
// adapter exists because Go has no covariant return types: *podman.Client.Create
// returns a concrete container and so cannot satisfy ContainerRuntime directly.
func PodmanRuntime(client *podman.Client) ContainerRuntime {
	return podmanAdapter{client: client}
}

type podmanAdapter struct{ client *podman.Client }

func (a podmanAdapter) EnsureImage(ctx context.Context, image string) (bool, error) {
	return a.client.EnsureImage(ctx, image)
}

func (a podmanAdapter) RemoveByName(ctx context.Context, name string) error {
	return a.client.RemoveByName(ctx, name)
}

func (a podmanAdapter) Create(ctx context.Context, rc podman.RunConfig) (RuntimeContainer, error) {
	return a.client.Create(ctx, rc)
}

// Engine holds the collaborators needed to process a commit.
type Engine struct {
	cfg   *config.Config
	store *store.Store
	s3    *storage.S3
	// outputs publishes the files a submission generated. It is the same object
	// storage client as s3, behind the small interface the upload path needs.
	outputs generatedFileStore
	podman  ContainerRuntime
	hub     *events.Hub
	logger  *slog.Logger
}

// New builds an Engine.
func New(cfg *config.Config, st *store.Store, s3 *storage.S3, rt ContainerRuntime, hub *events.Hub, logger *slog.Logger) *Engine {
	engine := &Engine{cfg: cfg, store: st, s3: s3, podman: rt, hub: hub, logger: logger}
	if s3 != nil {
		engine.outputs = s3
	}

	return engine
}

// runOutcome is the result of the container phase.
type runOutcome struct {
	// status is set only when the run terminated before grading
	// (currently: compilation_error).
	status             model.RunStatus
	compilationMessage string
	compilationError   string
	caseResults        []model.CaseResult
	// streamed holds the cases already graded and published while the suite was
	// still running, so the final pass neither grades nor publishes them again.
	streamed *caseStream
	// missingCollections counts the cases whose image did not collect generated
	// files. It is reported once per run: see errNoCollectionDir.
	missingCollections int
	// publisher uploads the generated files. It is closed by Process after the
	// terminal event, so the client is not kept waiting for the last upload.
	publisher *outputPublisher
}

/*
caseStream collects the results the judge publishes while the container is still
running the remaining cases. The judge drives the log stream from one goroutine,
so a plain map with a mutex is enough; the lock also lets gradeAll read it after
the run ends without a second path.
*/
type caseStream struct {
	mu      sync.Mutex
	results map[int64]model.CaseResult
}

func newCaseStream() *caseStream {
	return &caseStream{results: make(map[int64]model.CaseResult)}
}

func (s *caseStream) store(result model.CaseResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[result.TestCaseID] = result
}

func (s *caseStream) lookup(testCaseID int64) (model.CaseResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[testCaseID]
	return result, ok
}

// caseDoneID parses `case.done <id> <nonce>`; the nonce is what proves the
// harness printed the line, since the submission's own output never reaches the
// container log.
func caseDoneID(line, nonce string) (int64, bool) {
	fields := strings.Fields(line)
	if len(fields) != 3 || fields[0] != "case.done" || fields[2] != nonce {
		return 0, false
	}
	id, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// Process runs one commit. It never panics out and always publishes a terminal
// `finished` event (except when the process is torn down mid-run).
func (e *Engine) Process(ctx context.Context, commit *model.Commit) {
	startedAt := time.Now()
	logger := e.logger.With("commit_id", commit.ID)
	timings := model.NewRunTimings()

	// The store records how long the row waited in the queue, measured on the
	// database clock in the claim transaction.
	if commit.HasQueueWait {
		timings.Record(model.PhaseQueue, commit.QueueWait)
	}

	var ws *workspace

	// Recover from panics to ensure we always send a finished event and clean up the workspace.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic while processing commit", "panic", r)

			// Report the panic as an internal error to the event hub.
			e.hub.Error(commit.ID, fmt.Sprintf("internal error: %v", r))
			e.finish(commit.ID, timings, model.RunServerError, 0, 0, "", "", startedAt)
		}

		// Clean up the workspace and the private expected-output directory if they
		// were created and the configuration does not require keeping workspaces.
		if ws != nil && !e.cfg.KeepWorkspaces {
			if err := os.RemoveAll(ws.BaseDir); err != nil {
				logger.Warn("workspace cleanup failed", "error", err)
			}
			if err := os.RemoveAll(ws.ExpectedDir); err != nil {
				logger.Warn("expected output cleanup failed", "error", err)
			}
		}
	}()

	e.hub.Status(commit.ID, "compiling", startedAt)

	var err error

	// Prepare the workspace for the commit, which includes setting up the necessary files and directories.
	prepareCtx, prepareSpan := telemetry.Tracer().Start(ctx, "judge.prepare_workspace",
		trace.WithAttributes(
			attribute.Int64("runcodes.commit.id", commit.ID),
			attribute.Int64("runcodes.exercise.id", commit.ExerciseID),
		),
	)
	ws, err = e.prepareWorkspace(prepareCtx, commit, timings)
	if err != nil {
		prepareSpan.RecordError(err)
		prepareSpan.SetStatus(codes.Error, "workspace preparation failed")
	}
	prepareSpan.End()
	if err != nil {
		logger.Error("workspace preparation failed", "error", err)
		e.hub.Error(commit.ID, err.Error())
		e.finish(commit.ID, timings, model.RunServerError, 0, 0, "", "", startedAt)
		return
	}

	// Check if the language for the commit could be determined. If not, log an error and finish the run with a server error status.
	if ws.Language == nil {
		msg := fmt.Sprintf("could not determine language for %q", commit.Filename())
		logger.Error(msg)
		e.hub.Error(commit.ID, msg)
		e.finish(commit.ID, timings, model.RunServerError, 0, 0, "", "", startedAt)
		return
	}

	// Run the container for the commit, which includes compilation and test execution.
	// Capture the outcome and any errors that occur during this process.
	runCtx, runSpan := telemetry.Tracer().Start(ctx, "judge.run_container",
		trace.WithAttributes(
			attribute.Int64("runcodes.commit.id", commit.ID),
			attribute.Int64("runcodes.exercise.id", commit.ExerciseID),
			attribute.String("runcodes.language", ws.Language.Name),
			attribute.Int("runcodes.test_case.count", len(ws.TestCases)),
		),
	)
	outcome, runErr := e.runContainer(runCtx, commit, ws, timings, logger)

	// Drain the artifact uploads before the terminal event. The ready/failed
	// events then always arrive while the client is still listening, so it never
	// has to poll the listing for an outcome the stream can no longer deliver.
	publisher := outcome.publisher
	closePublisher := func() {
		if publisher != nil {
			publisher.Close()
			publisher = nil
		}
	}
	defer closePublisher()
	if outcome.missingCollections > 0 {
		logger.Warn("the runner image did not collect generated files; rebuild the runner images",
			"image", ws.Language.Image(e.cfg.ImageFormat),
			"cases", outcome.missingCollections,
		)
	}

	if runErr != nil {
		runSpan.RecordError(runErr)
		runSpan.SetStatus(codes.Error, "container run failed")
	} else {
		runSpan.SetAttributes(
			attribute.String("runcodes.status", string(outcome.status)),
			attribute.Bool("runcodes.compilation_error", outcome.status == model.RunCompilationError),
			attribute.Int("runcodes.graded_cases", len(outcome.caseResults)),
		)
	}
	runSpan.End()

	if runErr != nil {
		// Classify the error to determine the appropriate run status (e.g., timeout, server error).
		status := classify(runErr)

		// A container that never reached its first milestone is an
		// infrastructure failure the administrators have to see: the student can
		// retry, and every retry will fail the same way until somebody looks.
		var startup *startupError
		if errors.As(runErr, &startup) {
			e.alertStartupFailure(commit, ws, startup, logger)
		}

		logger.Warn("run failed", "status", status, "error", runErr)
		closePublisher()
		e.hub.Error(commit.ID, runErr.Error())
		e.finish(commit.ID, timings, status, 0, 0,
			outcome.compilationMessage, outcome.compilationError, startedAt)
		return
	}

	// Report the results of each test case to the event hub. Cases already
	// published when their milestone arrived keep their single event.
	for _, r := range outcome.caseResults {
		if outcome.streamed != nil {
			if _, published := outcome.streamed.lookup(r.TestCaseID); published {
				continue
			}
		}
		e.hub.CaseResult(commit.ID, r.TestCaseID, r.CPUTime, r.WallTime, r.MemUsage,
			string(r.Status), string(r.Reason), r.StatusMsg, r.UserOutput,
			r.OutputType, r.ErrorMessage)
	}

	// Every per-case upload finishes before the run's own files and the terminal
	// event. A late ready event would otherwise be dropped by the hub.
	closePublisher()

	// Publish whatever the submission wrote straight into the runner's output
	// directory. The per-case files were already uploaded as their cases
	// finished. Measure the step even when there is nothing to upload: the phase
	// still tells us it ran.
	artifactDone := timings.Measure(model.PhaseArtifact)
	uploaded, artifactErr := e.publishRootOutputs(ctx, commit.ID, ws)
	artifactDone()
	if artifactErr != nil {
		logger.Warn("could not upload generated files", "error", artifactErr)
	}
	if uploaded > 0 {
		e.hub.Artifact(commit.ID, "output",
			fmt.Sprintf("/api/v1/submissions/%d/outputfiles", commit.ID))
	}

	if outcome.status == model.RunCompilationError {
		e.finish(commit.ID, timings, model.RunCompilationError, 0, 0,
			outcome.compilationMessage, outcome.compilationError, startedAt)
		return
	}

	correct, score, status := scoreRun(ws.TestCases, outcome.caseResults)

	e.finish(commit.ID, timings, status, correct, score,
		outcome.compilationMessage, outcome.compilationError, startedAt)

	logger.Info("commit processed", "status", status, "correct", correct, "score", score)
}

// finish publishes the run's phase timings and then the terminal event. Keeping
// both in one call is what makes the phase data survive every terminal path,
// including compilation errors and panics.
func (e *Engine) finish(
	id int64, timings *model.RunTimings, status model.RunStatus,
	correct int, score float64, message, errMsg string, startedAt time.Time,
) {
	e.hub.Timings(id, timings.Phases())
	e.hub.Finished(id, string(status), correct, score, message, errMsg,
		startedAt, time.Now())
}

// secondsDuration converts the monitor's float seconds to a time.Duration.
func secondsDuration(seconds float64) time.Duration {
	if seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// runContainer creates, starts and drives one container to completion.
func (e *Engine) runContainer(
	ctx context.Context, commit *model.Commit, ws *workspace,
	timings *model.RunTimings, logger *slog.Logger,
) (runOutcome, error) {
	var outcome runOutcome

	image := ws.Language.Image(e.cfg.ImageFormat)
	imageStartedAt := time.Now()
	pulled, imageErr := e.podman.EnsureImage(ctx, image)
	timings.RecordDetail(model.PhaseImageEnsure, time.Since(imageStartedAt),
		map[string]any{"image": image, "pulled": pulled})
	if imageErr != nil {
		return outcome, fmt.Errorf("ensure image: %w", imageErr)
	}

	// The container name is derived from the commit ID to ensure uniqueness.
	// If a container with the same name exists (e.g., due to a previous crash), it is removed before creating a new one.
	name := fmt.Sprintf("runcodes-%d", commit.ID)
	createDone := timings.Measure(model.PhaseContainerCreate)
	_ = e.podman.RemoveByName(ctx, name)

	ct, err := e.podman.Create(ctx, podman.RunConfig{
		Name:        name,
		Image:       image,
		MountSource: ws.RemoteDir,
		Labels:      map[string]string{podman.CommitLabel: fmt.Sprintf("%d", commit.ID)},
	})
	createDone()
	if err != nil {
		return outcome, err
	}

	startDone := timings.Measure(model.PhaseContainerStart)
	startErr := ct.Start(ctx)
	startDone()
	if startErr != nil {
		_ = ct.Remove(context.Background())
		return outcome, startErr
	}

	startupStartedAt := time.Now()
	stream := ct.Logs(ctx)

	// startupStart measures container boot plus harness initialization up to the
	// first phase milestone. For compilable languages that is
	// `compilation.start`; for interpreted ones it is `run.start`, because the
	// image script skips the compile function entirely.
	startupRecorded := false
	recordStartup := func() {
		if startupRecorded {
			return
		}
		startupRecorded = true
		timings.Record(model.PhaseStartup, time.Since(startupStartedAt))
	}

	// The final grading pass must run after the container is stopped: the
	// container runs the untrusted submission, so while it is alive it could
	// still rewrite the output files the judge is about to read (and, because the
	// milestones below are plain log lines, it can claim the execution phase is
	// over and keep running). Cases streamed below are graded as their milestone
	// arrives, while the suite continues, so only the cases that were not
	// streamed are read in the final pass. `stopped` tracks whether stopContainer
	// already ran, so the deferred call only covers the error paths that return
	// before it.
	var monitor *workspaceMonitor
	stopped := false
	defer func() {
		// Stop the container before the monitor: a background process can keep
		// writing until the container is killed, and the deferred stop is the
		// path every error takes.
		if !stopped {
			_, _ = e.stopContainer(ct, stream)
		}
		if monitor != nil {
			monitor.stop()
		}
	}()

	// Bound the aggregate workspace while untrusted code is running. The monitor
	// kills the container on a size or inode overage; the checks below turn the
	// resulting stream failure into the specific quota error.
	monitor = e.startWorkspaceMonitor(ctx, ws.BaseDir, ct)

	// Await milestones from the container's log stream to determine the progress of compilation and execution.
	if ws.Compilable {
		// The first milestone is still startup: the container booting, the harness
		// starting and the line reaching the judge. CompilationWait begins after
		// it — and then it must outwait the container's own compilation limit, or
		// the judge abandons runs the image was still compiling (and reports a
		// timeout instead of the compiler's error).
		if err := e.awaitStartup(stream, "compilation.start", ws.RunNonce); err != nil {
			return outcome, workspaceAwareError(monitor, err)
		}
		recordStartup()

		// Await the compilation done milestone, which indicates that the compilation phase has completed.
		if err := e.awaitLine(stream, "compilation.done", ws.RunNonce, e.cfg.CompilationWait); err != nil {
			return outcome, workspaceAwareError(monitor, err)
		}

		outcome.compilationMessage = readLimited(filepath.Join(ws.BaseDir, "compilation.out"), e.cfg.MaxOutputFileSize)
		compileDiagnostics := readLimited(filepath.Join(ws.BaseDir, "compilation.err"), e.cfg.MaxOutputFileSize)
		compileWarnings := readLimited(filepath.Join(ws.BaseDir, "compilation.warnings"), e.cfg.MaxOutputFileSize)

		// The compiler's exit status is authoritative. A compiler can fail while
		// writing nothing to stderr (or be killed by `timeout`, which reports
		// status 124), so a missing or non-zero status is a compilation error.
		// On success, stderr is only diagnostics: warnings about otherwise valid
		// code must not fail a submission.
		compileInfo := readMonitor(filepath.Join(ws.BaseDir, "compilation.monitor_out"))
		if compileInfo.HasTime {
			timings.RecordMonitor(model.PhaseCompile,
				secondsDuration(compileInfo.Time),
				secondsDuration(compileInfo.CPU),
				compileInfo.HasCPU,
				compileInfo.Mem, compileInfo.HasMem)
		}
		compileFailed := compilationFailed(compileInfo)

		if compileFailed {
			// Let a quota kill that raced with compilation take precedence over
			// the compiler diagnostics. The deferred cleanup stops the container
			// before it stops the monitor.
			if monitor != nil {
				if limitErr := monitor.Err(); limitErr != nil {
					return outcome, limitErr
				}
			}

			outcome.compilationError = compileDiagnostics
			if outcome.compilationError == "" {
				reason := "the compiler did not exit successfully"
				if !compileInfo.Valid() {
					reason = "the monitor did not produce a report"
				} else if msg, failed := exitFailureMessage(compileInfo); failed {
					reason = msg
				}
				outcome.compilationError = fmt.Sprintf("compilation failed: %s", reason)
			}
			outcome.status = model.RunCompilationError
			e.hub.Compilation(commit.ID, false, outcome.compilationMessage, outcome.compilationError, time.Now())
			logger.Info("compilation failed")
			return outcome, nil
		}

		// Older harnesses leave diagnostics in compilation.err even on success.
		// Treat them as warnings so the student still sees them without failing
		// the run.
		if compileWarnings == "" {
			compileWarnings = compileDiagnostics
		}
		if compileWarnings != "" {
			if strings.TrimSpace(outcome.compilationMessage) != "" {
				outcome.compilationMessage += "\n"
			}
			outcome.compilationMessage += "compiler warnings:\n" + compileWarnings
		}
	}

	// If compilation was successful (or not required), report the successful compilation to the event hub.
	e.hub.Compilation(commit.ID, true, outcome.compilationMessage, "", time.Now())

	// Await the run start milestone, which indicates that the execution phase has begun.
	// For an interpreted language this is the first milestone, so it is still
	// bounded by the startup budget; the execution timeout applies from here on.
	timeout := e.executionTimeout(ws.TestCases)
	if err := e.awaitStartup(stream, "run.start", ws.RunNonce); err != nil {
		return outcome, workspaceAwareError(monitor, err)
	}
	// No-op for compilable languages, which recorded startup at compilation.start.
	recordStartup()
	runStartedAt := time.Now()

	// Report to the event hub that the run phase is now running.
	e.hub.Status(commit.ID, "running", time.Now())

	streamed := newCaseStream()
	outcome.streamed = streamed

	// Uploads run in the background: grading and reading the next milestone must
	// not wait for a large file to reach object storage.
	publisher := newOutputPublisher(ctx, e, commit.ID)
	outcome.publisher = publisher

	onLine := func(line string) {
		testCaseID, ok := caseDoneID(line, ws.RunNonce)
		if !ok {
			return
		}
		testCase, found := findTestCase(ws.TestCases, testCaseID)
		if !found {
			return
		}
		if _, done := streamed.lookup(testCaseID); done {
			return
		}
		result, gradeErr := e.gradeCase(ctx, testCase, ws)
		if gradeErr != nil {
			// The final pass grades it again; a case that cannot be graded here
			// must not take the run down.
			logger.Warn("could not grade a finished test case while the run continued",
				"test_case_id", testCaseID,
				"error", gradeErr,
			)
			return
		}
		streamed.store(result)

		// The result goes out before the files: it is ready now, and the upload must
		// not delay it.
		e.hub.CaseResult(commit.ID, result.TestCaseID, result.CPUTime,
			result.WallTime, result.MemUsage, string(result.Status),
			string(result.Reason), result.StatusMsg, result.UserOutput,
			result.OutputType, result.ErrorMessage)

		// Queue the files this case produced while the suite continues: the
		// workspace then holds one case's files at most, instead of the whole
		// suite's, and the student can already download them.
		if err := e.queueCaseOutputs(publisher, commit.ID, testCase, result, ws); err != nil {
			if errors.Is(err, errNoCollectionDir) {
				// Reported once, after the run: logging per case would repeat the
				// same line for every test in the exercise.
				outcome.missingCollections++
			} else {
				logger.Warn("could not publish a finished case's generated files",
					"test_case_id", testCaseID,
					"error", err,
				)
			}
		}

		// The harness removes the case's scratch directory itself, after its
		// own copies; doing it here too means an older image, or a harness that
		// died before its cleanup line, cannot leave the workspace growing case
		// after case until the quota kills the run. Grading already happened:
		// everything it needs is at the workspace root.
		if err := os.RemoveAll(filepath.Join(ws.BaseDir, "test_"+strconv.FormatInt(testCaseID, 10))); err != nil {
			logger.Warn("could not remove a finished case's scratch directory",
				"test_case_id", testCaseID, "error", err)
		}
	}
	if err := e.awaitLine(stream, "run.done", ws.RunNonce, timeout, onLine); err != nil {
		return outcome, workspaceAwareError(monitor, err)
	}
	timings.Record(model.PhaseRun, time.Since(runStartedAt))

	// Execution is over: kill and remove the container before reading anything it
	// produced, so no process of the submission's can still be running while the
	// judge grades it. The exit status is the harness's own verdict on itself.
	exitCode, exited := e.stopContainer(ct, stream)
	stopped = true

	// The container is gone, so no process can grow the workspace any further.
	// Preserve an overage detected while it was alive before grading.
	if monitor != nil {
		monitor.stop()
		if limitErr := monitor.Err(); limitErr != nil {
			return outcome, limitErr
		}
	}

	// A harness that failed after reporting the run as done — a broken image, a
	// container killed from outside, a full disk — has not finished collecting its
	// outputs, so grading them would invent a result.
	if exited && exitCode != 0 {
		return outcome, fmt.Errorf("the container's run harness exited with status %d", exitCode)
	}

	gradeDone := timings.Measure(model.PhaseGrade)
	results, err := e.gradeAll(ctx, commit, ws, streamed)
	gradeDone()
	if err != nil {
		return outcome, err
	}

	outcome.caseResults = results
	return outcome, nil
}

// executionTimeout calculates the total timeout for the execution phase based on
// the number of test cases and their individual CPU time limits. It returns the
// total duration to be used as a timeout for awaiting milestones in the container's log stream.
func (e *Engine) executionTimeout(cases []model.TestCase) time.Duration {
	total := e.cfg.BaseExecTimeout * time.Duration(1+len(cases))

	for _, tc := range cases {
		// If the test case has a CPU time limit, use it; otherwise, use the default case timeout.
		d := time.Duration(tc.CPUTimeLimit) * time.Second

		if tc.CPUTimeLimit <= 0 {
			d = e.cfg.DefaultCaseTimeout
		}

		total += d
	}

	return total
}

// classify maps a run failure to a terminal status.
func classify(err error) model.RunStatus {
	var (
		timedOut *timeoutError
		startup  *startupError
	)

	// A container that never started is never the submission's fault: nothing it
	// wrote has run yet, so a `timeout` here would blame the student for the
	// runtime or the judge. It is checked first because the startup budget
	// produces a timeout of its own.
	if errors.As(err, &startup) {
		return model.RunServerError
	}

	// Only the phase timeouts the runner imposed itself are the submission's own
	// timeout. A cancelled or expired run context belongs to the judge's lifecycle
	// (shutdown, or the per-run budget), so reporting it as the student's TLE would
	// penalize them for an infrastructure event.
	if errors.As(err, &timedOut) {
		return model.RunTimeout
	}

	return model.RunServerError
}

/*
alertStartupFailure publishes an `alert` event for a run the container never
started, so the administrators learn about it from the platform itself.

The event is published before the terminal `finished` (the hub closes the topic
then), and the backend records it once per (commit, code) and emails every
administrator. The message names the commit and the language because that is
what an administrator needs to correlate it with the submission.
*/
func (e *Engine) alertStartupFailure(
	commit *model.Commit, ws *workspace, startup *startupError, logger *slog.Logger,
) {
	image := ws.Language.Image(e.cfg.ImageFormat)
	message := fmt.Sprintf(
		"the %s container for commit %d did not print %q within %s; the submission could not be graded",
		ws.Language.Name, commit.ID, startup.Milestone, startup.Wait,
	)

	logger.Error("container startup budget exceeded",
		"milestone", startup.Milestone,
		"wait", startup.Wait,
		"image", image,
	)

	e.hub.Alert(
		commit.ID,
		model.AlertSeverityCritical,
		model.AlertStartupTimeout,
		message,
		map[string]any{
			"language":     ws.Language.Name,
			"image":        image,
			"filename":     commit.Filename(),
			"milestone":    startup.Milestone,
			"wait_seconds": startup.Wait.Seconds(),
			"test_cases":   len(ws.TestCases),
		},
	)
}

/*
isInternalOutputFile reports whether a regular file in outputfiles is one of the
runner's own reports rather than a file produced by the submission:
compilation.* metadata and `<case id>.{output,error,monitor_out,monitor_err}`.
*/
func isInternalOutputFile(name string) bool {
	base := filepath.Base(name)
	if strings.HasPrefix(base, "compilation.") {
		return true
	}

	ext := filepath.Ext(base)
	switch ext {
	case ".monitor_out", ".monitor_err", ".output", ".error":
		_, err := strconv.ParseInt(strings.TrimSuffix(base, ext), 10, 64)
		return err == nil
	default:
		return false
	}
}
