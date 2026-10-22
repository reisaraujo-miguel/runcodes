// Package engine runs a claimed commit end to end: prepare the workspace, drive
// the container through compilation and test execution, grade the outputs and
// publish the resulting events. It performs no persistence.
package engine

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	EnsureImage(ctx context.Context, image string) error
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

func (a podmanAdapter) EnsureImage(ctx context.Context, image string) error {
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
	cfg    *config.Config
	store  *store.Store
	s3     *storage.S3
	podman ContainerRuntime
	hub    *events.Hub
	logger *slog.Logger
}

// New builds an Engine.
func New(cfg *config.Config, st *store.Store, s3 *storage.S3, rt ContainerRuntime, hub *events.Hub, logger *slog.Logger) *Engine {
	return &Engine{cfg: cfg, store: st, s3: s3, podman: rt, hub: hub, logger: logger}
}

// ArtifactPath is where a commit's output archive lives (and is served from).
func (e *Engine) ArtifactPath(commitID int64) string {
	return filepath.Join(e.cfg.ExecDir, "outputs", fmt.Sprintf("%d.zip", commitID))
}

// runOutcome is the result of the container phase.
type runOutcome struct {
	// status is set only when the run terminated before grading
	// (currently: compilation_error).
	status             model.RunStatus
	compilationMessage string
	compilationError   string
	caseResults        []model.CaseResult
}

// Process runs one commit. It never panics out and always publishes a terminal
// `finished` event (except when the process is torn down mid-run).
func (e *Engine) Process(ctx context.Context, commit *model.Commit) {
	startedAt := time.Now()
	logger := e.logger.With("commit_id", commit.ID)

	var ws *workspace

	// Recover from panics to ensure we always send a finished event and clean up the workspace.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic while processing commit", "panic", r)

			// Report the panic as an internal error to the event hub.
			e.hub.Error(commit.ID, fmt.Sprintf("internal error: %v", r))
			e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
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
	ws, err = e.prepareWorkspace(prepareCtx, commit)
	if err != nil {
		prepareSpan.RecordError(err)
		prepareSpan.SetStatus(codes.Error, "workspace preparation failed")
	}
	prepareSpan.End()
	if err != nil {
		logger.Error("workspace preparation failed", "error", err)
		e.hub.Error(commit.ID, err.Error())
		e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
		return
	}

	// Check if the language for the commit could be determined. If not, log an error and finish the run with a server error status.
	if ws.Language == nil {
		msg := fmt.Sprintf("could not determine language for %q", commit.Filename())
		logger.Error(msg)
		e.hub.Error(commit.ID, msg)
		e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
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
	outcome, runErr := e.runContainer(runCtx, commit, ws, logger)
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

		logger.Warn("run failed", "status", status, "error", runErr)
		e.hub.Error(commit.ID, runErr.Error())
		e.hub.Finished(commit.ID, string(status), 0, 0,
			outcome.compilationMessage, outcome.compilationError, startedAt, time.Now())
		return
	}

	// Report the results of each test case to the event hub.
	for _, r := range outcome.caseResults {
		e.hub.CaseResult(commit.ID, r.TestCaseID, r.CPUTime, r.MemUsage,
			string(r.Status), r.StatusMsg, r.UserOutput, r.OutputType, r.ErrorMessage)
	}

	// Build an artifact (zip archive) of the output files, if any, and report its availability to the event hub.
	if zipPath, err := e.buildArtifact(commit.ID, ws); err != nil {
		logger.Warn("could not build output archive", "error", err)
	} else if zipPath != "" {
		e.hub.Artifact(commit.ID, "output", fmt.Sprintf("/v1/runs/%d/output", commit.ID))
	}

	if outcome.status == model.RunCompilationError {
		e.hub.Finished(commit.ID, string(model.RunCompilationError), 0, 0,
			outcome.compilationMessage, outcome.compilationError, startedAt, time.Now())
		return
	}

	correct, score, status := scoreRun(ws.TestCases, outcome.caseResults)

	e.hub.Finished(commit.ID, string(status), correct, score,
		outcome.compilationMessage, outcome.compilationError, startedAt, time.Now())

	logger.Info("commit processed", "status", status, "correct", correct, "score", score)
}

// runContainer creates, starts and drives one container to completion.
func (e *Engine) runContainer(ctx context.Context, commit *model.Commit, ws *workspace, logger *slog.Logger) (runOutcome, error) {
	var outcome runOutcome

	image := ws.Language.Image(e.cfg.ImageFormat)
	if err := e.podman.EnsureImage(ctx, image); err != nil {
		return outcome, fmt.Errorf("ensure image: %w", err)
	}

	// The container name is derived from the commit ID to ensure uniqueness.
	// If a container with the same name exists (e.g., due to a previous crash), it is removed before creating a new one.
	name := fmt.Sprintf("runcodes-%d", commit.ID)
	_ = e.podman.RemoveByName(ctx, name)

	ct, err := e.podman.Create(ctx, podman.RunConfig{
		Name:        name,
		Image:       image,
		MountSource: ws.RemoteDir,
		Labels:      map[string]string{"io.runcodes.commit": fmt.Sprintf("%d", commit.ID)},
	})
	if err != nil {
		return outcome, err
	}

	if err := ct.Start(ctx); err != nil {
		_ = ct.Remove(context.Background())
		return outcome, err
	}

	stream := ct.Logs(ctx)

	// The container must be stopped before grading, not after: it runs the
	// untrusted submission, so while it is alive it can still rewrite the output
	// files the judge is about to read (and, because the milestones below are
	// plain log lines, it can claim the execution phase is over and keep running).
	// stopped tracks whether stopContainer already ran, so the deferred call only
	// covers the error paths that return before it.
	stopped := false
	defer func() {
		if !stopped {
			_, _ = e.stopContainer(ct, stream)
		}
	}()

	// Await milestones from the container's log stream to determine the progress of compilation and execution.
	if ws.Compilable {
		// If the language requires compilation, wait for the compilation start and done milestones.
		// CompilationWait, not the container's compilation timeout: the judge has to
		// outwait the container's own limit, or it abandons runs the image was still
		// compiling (and reports a timeout instead of the compiler's error).
		if err := e.awaitLine(stream, "compilation.start", ws.RunNonce, e.cfg.CompilationWait); err != nil {
			return outcome, err
		}

		// Await the compilation done milestone, which indicates that the compilation phase has completed.
		if err := e.awaitLine(stream, "compilation.done", ws.RunNonce, e.cfg.CompilationWait); err != nil {
			return outcome, err
		}

		outcome.compilationMessage = readLimited(filepath.Join(ws.BaseDir, "compilation.out"), e.cfg.MaxOutputFileSize)
		outcome.compilationError = readLimited(filepath.Join(ws.BaseDir, "compilation.err"), e.cfg.MaxOutputFileSize)

		// If there was a compilation error, set the outcome status to RunCompilationError and report it to the event hub.
		if outcome.compilationError != "" {
			outcome.status = model.RunCompilationError
			e.hub.Compilation(commit.ID, false, outcome.compilationMessage, outcome.compilationError, time.Now())
			logger.Info("compilation failed")
			return outcome, nil
		}
	}

	// If compilation was successful (or not required), report the successful compilation to the event hub.
	e.hub.Compilation(commit.ID, true, outcome.compilationMessage, "", time.Now())

	// Await the run start milestone, which indicates that the execution phase has begun.
	timeout := e.executionTimeout(ws.TestCases)
	if err := e.awaitLine(stream, "run.start", ws.RunNonce, timeout); err != nil {
		return outcome, err
	}

	// Report to the event hub that the run phase is now running.
	e.hub.Status(commit.ID, "running", time.Now())

	if err := e.awaitLine(stream, "run.done", ws.RunNonce, timeout); err != nil {
		return outcome, err
	}

	// Execution is over: kill and remove the container before reading anything it
	// produced, so no process of the submission's can still be running while the
	// judge grades it. The exit status is the harness's own verdict on itself.
	exitCode, exited := e.stopContainer(ct, stream)
	stopped = true

	// A harness that failed after reporting the run as done — a broken image, a
	// container killed from outside, a full disk — has not finished collecting its
	// outputs, so grading them would invent a result.
	if exited && exitCode != 0 {
		return outcome, fmt.Errorf("the container's run harness exited with status %d", exitCode)
	}

	results, err := e.gradeAll(ctx, commit, ws)
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
	var timedOut *timeoutError

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

// buildArtifact zips the files the submission produced. It returns "" when
// there is nothing to archive.
//
// Two places can contain user output:
//   - `outputfiles`, where the runner copies its own reports. Those reports are
//     filtered out; anything else the submission wrote there is kept.
//   - each case's `test_<id>` working directory, which is also where the source
//     and test-case files were copied. Files whose path is a known input are
//     skipped; newly created regular files are archived under `test_<id>/`.
//
// The workspace is written by untrusted code, so only regular files are read
// (never a symlink, which would otherwise publish the contents of any file the
// judge can read), and the archive stops at MaxArtifactBytes so a submission
// cannot fill the shared execution directory.
func (e *Engine) buildArtifact(commitID int64, ws *workspace) (string, error) {
	baseDir := ws.BaseDir
	outputDir := filepath.Join(baseDir, "outputfiles")

	type candidate struct {
		path string
		name string
	}

	var (
		files    []candidate
		archived int64
	)

	add := func(path, name string, fi os.FileInfo) error {
		if archived+fi.Size() > e.cfg.MaxArtifactBytes {
			return fmt.Errorf("output archive exceeds the %d byte limit", e.cfg.MaxArtifactBytes)
		}
		archived += fi.Size()
		files = append(files, candidate{path: path, name: name})
		return nil
	}

	// Files the submission wrote directly into outputfiles. The runner also
	// copies its own per-case reports and compiler metadata there; those are
	// internal bookkeeping, not user output.
	if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
		err := filepath.Walk(outputDir, func(p string, fi os.FileInfo, openErr error) error {
			if openErr != nil {
				return openErr
			}
			if fi.IsDir() {
				return nil
			}
			if !fi.Mode().IsRegular() {
				e.logger.Warn("skipping non-regular output file",
					"commit_id", commitID, "name", filepath.Base(p))
				return nil
			}
			if isInternalOutputFile(fi.Name()) {
				return nil
			}
			rel, err := filepath.Rel(outputDir, p)
			if err != nil {
				return err
			}
			return add(p, rel, fi)
		})
		if err != nil {
			return "", err
		}
	}

	sourceInputs := inputFiles(ws.SourceDir)

	// Files the submission created while running each test case. The working
	// directory also contains the source and case files copied in by the runner;
	// those are inputs, not output.
	for _, tc := range ws.TestCases {
		testName := fmt.Sprintf("test_%d", tc.ID)
		testDir := filepath.Join(baseDir, testName)

		info, err := os.Stat(testDir)
		if err != nil || !info.IsDir() {
			continue
		}

		caseInputs := make(map[string]struct{}, len(tc.Files))
		for _, name := range tc.Files {
			caseInputs[filepath.Base(name)] = struct{}{}
		}

		err = filepath.Walk(testDir, func(p string, fi os.FileInfo, openErr error) error {
			if openErr != nil {
				return openErr
			}
			if fi.IsDir() {
				return nil
			}
			if !fi.Mode().IsRegular() {
				e.logger.Warn("skipping non-regular output file",
					"commit_id", commitID, "name", filepath.Base(p))
				return nil
			}

			rel, err := filepath.Rel(testDir, p)
			if err != nil {
				return err
			}
			if _, ok := sourceInputs[filepath.ToSlash(rel)]; ok {
				return nil
			}
			if _, ok := caseInputs[filepath.Base(rel)]; ok {
				return nil
			}

			return add(p, filepath.Join(testName, rel), fi)
		})
		if err != nil {
			return "", err
		}
	}

	if len(files) == 0 {
		return "", nil
	}

	outputsDir := filepath.Join(e.cfg.ExecDir, "outputs")
	if err := os.MkdirAll(outputsDir, 0o777); err != nil {
		return "", err
	}

	zipPath := filepath.Join(outputsDir, fmt.Sprintf("%d.zip", commitID))

	f, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}

	zw := zip.NewWriter(f)

	cleanup := func() {
		_ = zw.Close()
		_ = f.Close()
		_ = os.Remove(zipPath)
	}

	for _, file := range files {
		w, err := zw.Create(filepath.ToSlash(file.name))
		if err != nil {
			cleanup()
			return "", err
		}

		src, err := openRegular(file.path)
		if err != nil {
			cleanup()
			return "", err
		}

		_, err = io.Copy(w, src)
		src.Close()
		if err != nil {
			cleanup()
			return "", err
		}
	}

	if err := zw.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(zipPath)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(zipPath)
		return "", err
	}

	return zipPath, nil
}

/*
inputFiles lists the relative paths of the files in the source directory. The
runner copies these into every test-case working directory, so they must not be
mistaken for files the submission generated.
*/
func inputFiles(dir string) map[string]struct{} {
	inputs := make(map[string]struct{})
	if dir == "" {
		return inputs
	}

	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err == nil {
			inputs[filepath.ToSlash(rel)] = struct{}{}
		}
		return nil
	})

	return inputs
}
