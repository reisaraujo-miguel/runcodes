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
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/runcodes-icmc/runcodes/cache"
	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
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
		slog.Error("Invalid configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// The flag is the local-development switch; DEBUG_MODE does the same for an
	// environment that cannot pass arguments to the binary. This is the only
	// change main makes to the configuration Load published.
	if *debugFlag {
		cfg.Debug = true
	}

	SetupLogger(cfg.Debug)
	cfg.LogInsecureTransportWarnings()

	if err := database.InitDB(); err != nil {
		slog.Error("Failed to initialize database")
		os.Exit(1)
	}
	defer database.DB.Close()

	if err := validation.SetupJWT(); err != nil {
		slog.Error("Failed to setup JWT", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := cache.Ping(context.Background()); err != nil {
		slog.Warn("Redis cache unavailable, continuing without it",
			slog.String("error", err.Error()),
		)
	}

	// start the judge reconciliation service in a separate goroutine
	go judge.StartReconciliation(context.Background())

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
		// purpose: the SSE submission stream is a long-lived response.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
