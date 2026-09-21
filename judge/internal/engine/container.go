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

// awaitLine consumes container log lines until `expected` appears, the timeout
// fires or the stream ends.
func (e *Engine) awaitLine(stream *podman.LogStream, expected string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timeout occurred while waiting for the expected line.
			return &timeoutError{msg: fmt.Sprintf("timed out waiting for %q", expected)}
		case line, ok := <-stream.Lines:
			// The log stream ended before the expected line was found.
			if !ok {
				return fmt.Errorf("%w before %q", errLogStreamEnded, expected)
			}

			// Check if the current line matches the expected line.
			if line == expected {
				return nil
			}

			e.logger.Debug("container output", "line", line)
		}
	}
}

// stopContainer waits for the container to exit, force-killing it on timeout,
// then removes it. Errors are logged, never fatal.
func (e *Engine) stopContainer(ct *podman.Container, stream *podman.LogStream) {
	defer stream.Close()

	// The run phases have finished reading milestones; keep draining so the
	// log reader goroutines can never block on a full buffer.
	go func() {
		for range stream.Lines {
		}
	}()

	// Wait for the container to exit, but don't block forever.
	if _, err := ct.Wait(e.cfg.BaseExecTimeout); err != nil {
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
}
