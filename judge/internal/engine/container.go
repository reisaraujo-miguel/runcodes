package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/runcodes-icmc/judge/internal/podman"
)

// timeoutError marks a milestone wait that exceeded its budget, so the runner
// can report the `timeout` terminal status instead of a generic failure.
type timeoutError struct{ msg string }

func (e *timeoutError) Error() string { return e.msg }

// errLogStreamEnded means the container exited before the expected milestone.
var errLogStreamEnded = errors.New("container log stream ended")

/*
startupError is a container that did not print its first milestone within the
judge's startup budget.

It is deliberately not a timeoutError: no submitted code has run at this point,
so the submission's own timeout is the wrong verdict. The run ends as
`server_error` and the runner publishes an alert for the administrators.
*/
type startupError struct {
	Milestone string
	Wait      time.Duration
}

func (e *startupError) Error() string {
	return fmt.Sprintf("the container did not print %q within %s", e.Milestone, e.Wait)
}

// milestoneLine is the exact line the image prints for a milestone: the name, a
// space and the run's nonce.
func milestoneLine(milestone, nonce string) string {
	if nonce == "" {
		return milestone
	}

	return milestone + " " + nonce
}

// awaitLine consumes container log lines until `expected` appears, the timeout
// fires or the stream ends.
//
// The milestone must carry the run's nonce. The submission's own output is
// interleaved in this stream, so a bare name is not evidence of anything: it is
// reported once as a warning, because the usual cause is an image older than the
// judge, and then ignored.
func (e *Engine) awaitLine(
	stream *podman.LogStream,
	expected, nonce string,
	timeout time.Duration,
	onLine ...func(string),
) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	wanted := milestoneLine(expected, nonce)
	warned := false

	for {
		select {
		case <-timer.C:
			// Timeout occurred while waiting for the expected line.
			return &timeoutError{msg: fmt.Sprintf("timed out waiting for %q", wanted)}
		case line, ok := <-stream.Lines:
			// The log stream ended before the expected line was found.
			if !ok {
				return fmt.Errorf("%w before %q", errLogStreamEnded, wanted)
			}

			// Check if the current line matches the expected line.
			if line == wanted {
				return nil
			}

			// The callback sees the milestones of a phase that has not ended
			// yet, which is how a case result is published while the rest of the
			// suite is still running.
			if len(onLine) > 0 && onLine[0] != nil {
				onLine[0](line)
			}

			if !warned && nonce != "" && line == expected {
				warned = true
				e.logger.Warn(
					"ignoring a milestone without the run nonce; the container image predates the milestone contract",
					"milestone", expected,
				)
			}

			e.logger.Debug("container output", "line", line)
		}
	}
}

/*
awaitStartup waits for the container's first milestone under the startup budget.

Every later milestone is measured against the budget of the phase it belongs to
(the compilation wait, or the execution timeout). This one waits for the
container to boot and for the harness to start printing, and it is the wait that
has to fail fast — with an infrastructure verdict, not the submission's timeout —
when the runtime, the image or the log stream is wedged.
*/
func (e *Engine) awaitStartup(stream *podman.LogStream, milestone, nonce string) error {
	wait := e.startupWait()
	err := e.awaitLine(stream, milestone, nonce, wait)

	var timedOut *timeoutError
	if errors.As(err, &timedOut) {
		return &startupError{Milestone: milestone, Wait: wait}
	}

	return err
}

// startupWait is the effective startup budget. The configuration validates it as
// positive; the fallback keeps an Engine built without one (tests) from failing
// every run against a zero budget.
func (e *Engine) startupWait() time.Duration {
	if e.cfg.StartupWait > 0 {
		return e.cfg.StartupWait
	}
	if e.cfg.CompilationWait > 0 {
		return e.cfg.CompilationWait
	}

	return time.Minute
}

// stopContainer waits for the container to exit, force-killing it on timeout,
// then removes it. It reports the container's exit code and whether the container
// exited on its own rather than being killed. Errors are logged, never fatal.
func (e *Engine) stopContainer(ct RuntimeContainer, stream *podman.LogStream) (int32, bool) {
	defer stream.Close()

	// The run phases have finished reading milestones; keep draining so the
	// log reader goroutines can never block on a full buffer.
	go func() {
		for range stream.Lines {
		}
	}()

	// Wait for the container to exit, but don't block forever.
	code, err := ct.Wait(e.cfg.BaseExecTimeout)
	exited := err == nil

	if err != nil {
		e.logger.Warn("container did not exit; killing", "error", err)

		// Force-kill the container if it didn't exit in time.
		if killErr := ct.Kill(context.Background()); killErr != nil {
			e.logger.Warn("could not kill container", "error", killErr)
		}
	}

	// Remove the container, logging any errors but not failing the run.
	if err := ct.Remove(context.Background()); err != nil {
		e.logger.Warn("container removal failed", "error", err)
	}

	return code, exited
}
