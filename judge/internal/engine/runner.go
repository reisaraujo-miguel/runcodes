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
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/model"
	"github.com/runcodes-icmc/judge/internal/podman"
	"github.com/runcodes-icmc/judge/internal/storage"
	"github.com/runcodes-icmc/judge/internal/store"
)

// Engine holds the collaborators needed to process a commit.
type Engine struct {
	cfg    *config.Config
	store  *store.Store
	s3     *storage.S3
	podman *podman.Client
	hub    *events.Hub
	logger *slog.Logger
}

// New builds an Engine.
func New(cfg *config.Config, st *store.Store, s3 *storage.S3, pc *podman.Client, hub *events.Hub, logger *slog.Logger) *Engine {
	return &Engine{cfg: cfg, store: st, s3: s3, podman: pc, hub: hub, logger: logger}
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

		// Clean up the workspace if it was created and the configuration does not require keeping workspaces.
		if ws != nil && !e.cfg.KeepWorkspaces {
			if err := os.RemoveAll(ws.BaseDir); err != nil {
				logger.Warn("workspace cleanup failed", "error", err)
			}
		}
	}()

	e.hub.Status(commit.ID, "compiling", startedAt)

	var err error

	// Prepare the workspace for the commit, which includes setting up the necessary files and directories.
	ws, err = e.prepareWorkspace(ctx, commit)
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
	outcome, runErr := e.runContainer(ctx, commit, ws, logger)
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
	if zipPath, err := e.buildArtifact(commit.ID, ws.BaseDir); err != nil {
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
	defer e.stopContainer(ct, stream)

	// Await milestones from the container's log stream to determine the progress of compilation and execution.
	if ws.Compilable {
		// If the language requires compilation, wait for the compilation start and done milestones.
		if err := e.awaitLine(stream, "compilation.start", e.cfg.CompilationTimeout); err != nil {
			return outcome, err
		}

		// Await the compilation done milestone, which indicates that the compilation phase has completed.
		if err := e.awaitLine(stream, "compilation.done", e.cfg.CompilationTimeout); err != nil {
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
	if err := e.awaitLine(stream, "run.start", timeout); err != nil {
		return outcome, err
	}

	// Report to the event hub that the run phase is now running.
	e.hub.Status(commit.ID, "running", time.Now())

	if err := e.awaitLine(stream, "run.done", timeout); err != nil {
		return outcome, err
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

	if errors.As(err, &timedOut) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return model.RunTimeout
	}

	return model.RunServerError
}

// buildArtifact zips the container's `outputfiles` directory, if any. It
// returns "" when there is nothing to archive.
func (e *Engine) buildArtifact(commitID int64, baseDir string) (string, error) {
	outputDir := filepath.Join(baseDir, "outputfiles")

	info, err := os.Stat(outputDir)
	if err != nil || !info.IsDir() {
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

	// Walk the output directory and add each file to the zip archive.
	walkErr := filepath.Walk(outputDir, func(p string, fi os.FileInfo, openErr error) error {
		if openErr != nil {
			return openErr
		}

		// Skip directories; we only want to add files to the zip archive.
		if fi.IsDir() {
			return nil
		}

		// Compute the relative path of the file to be added to the zip archive.
		rel, err := filepath.Rel(outputDir, p)
		if err != nil {
			return err
		}

		// Create a new file in the zip archive with the relative path.
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}

		// Open the source file for reading and copy its contents to the zip archive.
		src, err := os.Open(p)
		if err != nil {
			return err
		}

		defer src.Close()

		_, err = io.Copy(w, src)

		return err
	})

	if err := zw.Close(); err != nil && walkErr == nil {
		walkErr = err
	}

	if err := f.Close(); err != nil && walkErr == nil {
		walkErr = err
	}

	if walkErr != nil {
		return "", walkErr
	}

	return zipPath, nil
}
