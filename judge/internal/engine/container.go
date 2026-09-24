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
func (e *Engine) awaitLine(stream *podman.LogStream, expected, nonce string, timeout time.Duration) error {
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
