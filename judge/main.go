// Command judge runs the run.codes judge service: a durable Postgres-backed
// queue of submissions executed in rootless podman containers, with results
// streamed to the backend over SSE.
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	s3, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.ExecDir, 0o777); err != nil {
		return err
	}

	pc := podman.New(cfg.PodmanURI)
	hub := events.New(cfg.EventRetention)
	eng := engine.New(cfg, st, s3, pc, hub, logger)
	pool := worker.New(cfg, st, eng, logger)

	go pool.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(cfg, st, pc, hub, eng, pool.Wake, logger).Router(),
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown error", "error", err)
	}
	// The worker pool observed the cancelled context and is draining; give
	// in-flight runs a moment to finish.
	time.Sleep(2 * time.Second)
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
