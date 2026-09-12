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
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic while processing commit", "panic", r)
			e.hub.Error(commit.ID, fmt.Sprintf("internal error: %v", r))
			e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
		}
		if ws != nil && !e.cfg.KeepWorkspaces {
			if err := os.RemoveAll(ws.BaseDir); err != nil {
				logger.Warn("workspace cleanup failed", "error", err)
			}
		}
	}()

	e.hub.Status(commit.ID, "compiling", startedAt)

	var err error
	ws, err = e.prepareWorkspace(ctx, commit)
	if err != nil {
		logger.Error("workspace preparation failed", "error", err)
		e.hub.Error(commit.ID, err.Error())
		e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
		return
	}
	if ws.Language == nil {
		msg := fmt.Sprintf("could not determine language for %q", commit.Filename())
		logger.Error(msg)
		e.hub.Error(commit.ID, msg)
		e.hub.Finished(commit.ID, string(model.RunServerError), 0, 0, "", "", startedAt, time.Now())
		return
	}

	outcome, runErr := e.runContainer(ctx, commit, ws, logger)
	if runErr != nil {
		status := classify(runErr)
		logger.Warn("run failed", "status", status, "error", runErr)
		e.hub.Error(commit.ID, runErr.Error())
		e.hub.Finished(commit.ID, string(status), 0, 0,
			outcome.compilationMessage, outcome.compilationError, startedAt, time.Now())
		return
	}

	for _, r := range outcome.caseResults {
		e.hub.CaseResult(commit.ID, r.TestCaseID, r.CPUTime, r.MemUsage,
			string(r.Status), r.StatusMsg, r.UserOutput, r.OutputType, r.ErrorMessage)
	}

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

	name := fmt.Sprintf("runcodes-%d", commit.ID)
	// A container orphaned by a previous crash would collide on the name.
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

	if ws.Compilable {
		if err := e.awaitLine(stream, "compilation.start", e.cfg.CompilationTimeout); err != nil {
			return outcome, err
		}
		if err := e.awaitLine(stream, "compilation.done", e.cfg.CompilationTimeout); err != nil {
			return outcome, err
		}
		outcome.compilationMessage = readLimited(filepath.Join(ws.BaseDir, "compilation.out"), e.cfg.MaxOutputFileSize)
		outcome.compilationError = readLimited(filepath.Join(ws.BaseDir, "compilation.err"), e.cfg.MaxOutputFileSize)
		if outcome.compilationError != "" {
			outcome.status = model.RunCompilationError
			e.hub.Compilation(commit.ID, false, outcome.compilationMessage, outcome.compilationError, time.Now())
			logger.Info("compilation failed")
			return outcome, nil
		}
	}
	e.hub.Compilation(commit.ID, true, outcome.compilationMessage, "", time.Now())

	timeout := e.executionTimeout(ws.TestCases)
	if err := e.awaitLine(stream, "run.start", timeout); err != nil {
		return outcome, err
	}
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

// executionTimeout bounds the whole run phase, mirroring the legacy budget:
// one base timeout per test case (plus the initial one) and the sum of the
// per-case CPU limits.
func (e *Engine) executionTimeout(cases []model.TestCase) time.Duration {
	total := e.cfg.BaseExecTimeout * time.Duration(1+len(cases))
	for _, tc := range cases {
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
	if errors.As(err, &timedOut) {
		return model.RunTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
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

	walkErr := filepath.Walk(outputDir, func(p string, fi os.FileInfo, openErr error) error {
		if openErr != nil {
			return openErr
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(outputDir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
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
