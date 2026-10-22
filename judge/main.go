/*
 * ░█████████                          ░██████                    ░██
 * ░██     ░██                        ░██   ░██                   ░██
 * ░██     ░██ ░██    ░██ ░████████  ░██         ░███████   ░████████  ░███████   ░███████
 * ░█████████  ░██    ░██ ░██    ░██ ░██        ░██    ░██ ░██    ░██ ░██    ░██ ░██
 * ░██   ░██   ░██    ░██ ░██    ░██ ░██        ░██    ░██ ░██    ░██ ░█████████  ░███████
 * ░██    ░██  ░██   ░███ ░██    ░██  ░██   ░██ ░██    ░██ ░██   ░███ ░██               ░██
 * ░██     ░██  ░█████░██ ░██    ░██   ░██████   ░███████   ░█████░██  ░███████   ░███████
 *
 * "Nobody's innocent, citizen. We're just here to determine the level of your guilt"
 *
 * -- Judge, Judge Dredd (1995)
 *
 */

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runcodes-icmc/judge/internal/api"
	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/engine"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/podman"
	"github.com/runcodes-icmc/judge/internal/storage"
	"github.com/runcodes-icmc/judge/internal/store"
	"github.com/runcodes-icmc/judge/internal/telemetry"
	"github.com/runcodes-icmc/judge/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("judge exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Listen for SIGINT and SIGTERM to gracefully shut down the server. The
	// context also cancels the telemetry setup and shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	telemetryShutdown, telemetryErr := telemetry.Setup(ctx)

	logger := slog.New(telemetry.NewLogHandler(telemetry.ServiceName(),
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}),
	))
	slog.SetDefault(logger)

	if telemetryErr != nil {
		logger.Warn("OpenTelemetry could not be initialized; continuing without it",
			"error", telemetryErr,
		)
		telemetryShutdown = func(context.Context) error { return nil }
	}
	if telemetry.Enabled() {
		logger.Info("OpenTelemetry enabled",
			"service", telemetry.ServiceName(),
			"endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		)
	} else {
		logger.Info("OpenTelemetry disabled (set OTEL_EXPORTER_OTLP_ENDPOINT to enable)")
	}
	defer shutdownTelemetry(telemetryShutdown)

	// Continuous profiling (for Alloy/Pyroscope) is opt-in and lives on a
	// separate internal listener, never the judge API.
	pprofShutdown, pprofErr := telemetry.StartPprof(os.Getenv("PPROF_ADDR"))
	if pprofErr != nil {
		logger.Warn("pprof server disabled", "error", pprofErr)
	} else if pprofShutdown != nil {
		defer shutdownPprof(pprofShutdown)
	}

	cfg.LogInsecureTransportWarnings()

	if cfg.AuthToken == "" {
		if cfg.AllowInsecureAPI {
			logger.Warn("JUDGE_AUTH_TOKEN is not set: the /v1 API is unauthenticated " +
				"(JUDGE_ALLOW_INSECURE=true); do not expose this instance to a network")
		} else {
			logger.Error("JUDGE_AUTH_TOKEN is not set: every /v1 request will be refused. " +
				"Set the token, or JUDGE_ALLOW_INSECURE=true for a local instance")
		}
	}

	// open the database connection pool and defer its closure
	st, err := store.New(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	// Queue depth and oldest-job age are async OTel gauges. Registering the
	// callback here keeps the data path independent of the worker loop and
	// lets the same metric work when the queue is empty.
	unregisterQueueMetrics, queueMetricsErr := telemetry.RegisterQueueMetrics(st)
	if queueMetricsErr != nil {
		logger.Warn("could not register judge queue metrics",
			"error", queueMetricsErr,
		)
	} else {
		defer unregisterQueueMetrics()
	}

	// open the S3 connection and defer its closure
	s3, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		return err
	}

	// ensure the execution directory exists and is writable
	if err := os.MkdirAll(cfg.ExecDir, 0o777); err != nil {
		return err
	}

	// Reclaim archives and abandoned workspaces left by previous runs. The
	// sweeper also runs hourly while the judge is up.
	engine.StartSweeper(ctx, cfg, logger)

	pc := podman.New(cfg.PodmanURI, podman.Limits{
		MemoryBytes: cfg.ContainerMemoryBytes,
		PidsLimit:   cfg.ContainerPidsLimit,
		CPUQuota:    cfg.ContainerCPUQuota,
	})

	// Probe podman once at startup. The judge still starts without it (the service
	// may come up later), but a socket this container cannot reach — one created or
	// restarted after the container was, most often — otherwise shows up only as a
	// refused submission long after the operator stopped looking at these logs.
	probeCtx, cancelProbe := context.WithTimeout(ctx, 5*time.Second)
	if err := pc.Ready(probeCtx); err != nil {
		logger.Warn("podman is not reachable: submissions are refused until it is",
			"error", err,
		)
	} else {
		pruneCtx, cancelPrune := context.WithTimeout(ctx, 30*time.Second)
		removed, err := pc.PruneLeftovers(pruneCtx, cfg.MaxRunDuration+5*time.Minute)
		cancelPrune()
		if err != nil {
			logger.Warn("could not prune leftover containers", "error", err)
		} else if removed > 0 {
			logger.Info("removed leftover containers from a previous run",
				"containers", removed,
			)
		}
	}
	cancelProbe()
	hub := events.New(cfg.EventRetention)
	eng := engine.New(cfg, st, s3, engine.PodmanRuntime(pc), hub, logger)
	pool := worker.New(cfg, st, eng, logger)

	// run the worker pool in a separate goroutine, which will process queued commits.
	// poolDone is closed once the pool has drained every in-flight run.
	poolDone := make(chan struct{})
	go func() {
		defer close(poolDone)
		pool.Run(ctx)
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(cfg, st, pc, eng, hub, pool.Wake, logger).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("judge listening",
			"addr", cfg.Addr,
			"concurrency", cfg.Concurrency,
			"podman", cfg.PodmanURI,
			"exec_dir", cfg.ExecDir,
		)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// The collector starts at the same time as the judge. A startup log emitted
	// before the collector's OTLP receiver is ready can be dropped, leaving the
	// judge with no log stream until the first submission. Emit one readiness
	// log after the startup race so operators always have a judge log to find.
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			logger.Info("judge ready for submissions")
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErr:
		return err
	}

	// shutting down the server

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Stop accepting new requests and wait for in-flight requests to finish.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown error", "error", err)
	}

	// The worker pool observed the cancelled context and is draining. Wait for
	// every in-flight run to finish, bounded by the shutdown deadline, instead of
	// abandoning runs that need longer than a fixed grace period.
	select {
	case <-poolDone:
	case <-shutdownCtx.Done():
		logger.Warn("shutdown deadline reached while draining the worker pool")
	}
	return nil
}

func logLevel() slog.Level {
	switch os.Getenv("JUDGE_LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// shutdownTelemetry gives the batch exporters a bounded chance to flush when the
// process receives SIGTERM. An unreachable collector must not delay shutdown.
func shutdownTelemetry(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		slog.Debug("telemetry shutdown error", "error", err)
	}
}

func shutdownPprof(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		slog.Debug("pprof shutdown error", "error", err)
	}
}
