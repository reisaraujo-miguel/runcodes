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
	"github.com/runcodes-icmc/judge/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("judge exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
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

	// Listen for SIGINT and SIGTERM to gracefully shut down the server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// open the database connection pool and defer its closure
	st, err := store.New(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	// open the S3 connection and defer its closure
	s3, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		return err
	}

	// ensure the execution directory exists and is writable
	if err := os.MkdirAll(cfg.ExecDir, 0o777); err != nil {
		return err
	}

	pc := podman.New(cfg.PodmanURI, podman.Limits{
		MemoryBytes: cfg.ContainerMemoryBytes,
		PidsLimit:   cfg.ContainerPidsLimit,
		CPUQuota:    cfg.ContainerCPUQuota,
	})
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
