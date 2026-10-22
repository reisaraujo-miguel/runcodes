/*  _______                                           _______
 * |       \                                         |       \
 * | $$$$$$$\ __    __  _______    _______   ______  | $$$$$$$\  ______    _______
 * | $$__| $$|  \  |  \|       \  /       \ /      \ | $$  | $$ /      \  /       \
 * | $$    $$| $$  | $$| $$$$$$$\|  $$$$$$$|  $$$$$$\| $$  | $$|  $$$$$$\|  $$$$$$$
 * | $$$$$$$\| $$  | $$| $$  | $$| $$      | $$  | $$| $$  | $$| $$    $$ \$$    \
 * | $$  | $$| $$__/ $$| $$  | $$| $$_____ | $$__/ $$| $$__/ $$| $$$$$$$$ _\$$$$$$\
 * | $$  | $$ \$$    $$| $$  | $$ \$$     \ \$$    $$| $$    $$ \$$     \|       $$
 *  \$$   \$$  \$$$$$$  \$$   \$$  \$$$$$$$  \$$$$$$  \$$$$$$$   \$$$$$$$ \$$$$$$$
 *
 * "Theory is when you know something but it doesn't work. Practice is when something
 *  works but you don't know why. At RunCodes we combine theory and practice: Nothing
 *  works and we don't know why."
 *
 *  -- Some Wise Developer
 *
 */

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/telemetry"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	// check if debug mode is enabled via command line flag
	debugFlag := flag.Bool("debug", false, "Sets the server to development mode")
	flag.Parse()

	// load environment variables from .env file if it exists, otherwise use system environment variables
	if err := godotenv.Load(); err != nil {
		slog.Info(
			"No .env file found, using environment variables",
			slog.String("error", err.Error()),
		)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// The flag is the local-development switch; DEBUG_MODE does the same for an
	// environment that cannot pass arguments to the binary. This is the only
	// change main makes to the configuration Load published.
	if *debugFlag {
		cfg.Debug = true
	}

	// Set up telemetry before the logger so the OTel log handler sees the
	// provider. Exporter failures are reported below and never stop the server:
	// observability is optional at runtime.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	telemetryShutdown, telemetryErr := telemetry.Setup(ctx)

	SetupLogger(cfg.Debug)
	cfg.LogInsecureTransportWarnings()

	if telemetryErr != nil {
		slog.Warn("OpenTelemetry could not be initialized; continuing without it",
			slog.String("error", telemetryErr.Error()),
		)
		telemetryShutdown = func(context.Context) error { return nil }
	}
	if telemetry.Enabled() {
		slog.Info("OpenTelemetry enabled",
			slog.String("service", telemetry.ServiceName()),
			slog.String("endpoint", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		)
	} else {
		slog.Info("OpenTelemetry disabled (set OTEL_EXPORTER_OTLP_ENDPOINT to enable)")
	}
	defer shutdownTelemetry(telemetryShutdown)

	// Continuous profiling (for Alloy/Pyroscope) is opt-in and lives on a
	// separate internal listener, never the public API.
	pprofShutdown, err := telemetry.StartPprof(os.Getenv("PPROF_ADDR"))
	if err != nil {
		slog.Warn("pprof server disabled", slog.String("error", err.Error()))
	} else if pprofShutdown != nil {
		defer shutdownPprof(pprofShutdown)
	}

	if err := database.InitDB(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer database.DB.Close()

	if err := validation.SetupJWT(); err != nil {
		return fmt.Errorf("setting up JWT: %w", err)
	}

	if err := cache.Ping(context.Background()); err != nil {
		slog.Warn("Redis cache unavailable, continuing without it",
			slog.String("error", err.Error()),
		)
	}

	// start the judge reconciliation service in a separate goroutine
	go judge.StartReconciliation(ctx)

	r := chi.NewRouter()
	configureMiddleware(r, cfg)
	createRoutes(r)

	if cfg.Debug {
		slog.Info("Server is running in debug mode", slog.String("port", cfg.Addr))
	} else {
		slog.Info("Server is running", slog.String("port", cfg.Addr))
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Addr),
		Handler: r,
		// Bound how long a client may take to send its request headers and how long
		// an idle keep-alive connection is held, so a slow-loris client cannot pin
		// a connection (and a goroutine) indefinitely. WriteTimeout stays unset on
		// purpose: the SSE submission stream is a long-lived response and sets its
		// own per-write deadline. Request bodies get a per-request read deadline in
		// limitRequestBody; a server-wide ReadTimeout would also cover long-lived SSE
		// connections, whose requests have no body to bound.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutdown requested")
	case err := <-serverErr:
		return fmt.Errorf("serving HTTP: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP shutdown error", slog.String("error", err.Error()))
	}

	return nil
}

// shutdownTelemetry gives the batch exporters a bounded chance to flush when the
// process receives SIGTERM. An unreachable collector must not delay shutdown.
func shutdownTelemetry(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		slog.Debug("telemetry shutdown error", slog.String("error", err.Error()))
	}
}

func shutdownPprof(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		slog.Debug("pprof shutdown error", slog.String("error", err.Error()))
	}
}
