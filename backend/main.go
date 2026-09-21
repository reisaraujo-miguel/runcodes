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

	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const debugModeEnv string = "DEBUG_MODE"

func main() {
	// check if debug mode is enabled via command line flag
	debugMode := flag.Bool("debug", false, "Sets the server to development mode")
	flag.Parse()

	if *debugMode {
		os.Setenv(debugModeEnv, "true")
	}

	// load environment variables from .env file if it exists, otherwise use system environment variables
	if err := godotenv.Load(); err != nil {
		slog.Info(
			"No .env file found, using environment variables",
			slog.String("error", err.Error()),
		)
	}

	// duh
	SetupLogger()

	var apiPort string
	if apiPort = os.Getenv("RUNCODES_API_PORT"); apiPort == "" {
		slog.Error("RUNCODES_API_PORT environment variable is not set")
		os.Exit(1)
	}

	if err := services.InitDB(); err != nil {
		slog.Error("Failed to initialize database")
		os.Exit(1)
	}
	defer services.DB.Close()

	if err := validation.SetupJWT(); err != nil {
		slog.Error("Failed to setup JWT", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := services.PingCache(context.Background()); err != nil {
		slog.Warn("Redis cache unavailable, continuing without it",
			slog.String("error", err.Error()),
		)
	}

	// start the judge reconciliation service in a separate goroutine
	go services.StartReconciliation(context.Background())

	r := chi.NewRouter()
	configureMiddleware(r)
	createRoutes(r)

	if os.Getenv(debugModeEnv) == "true" {
		slog.Info("Server is running in debug mode", slog.String("port", apiPort))
	} else {
		slog.Info("Server is running", slog.String("port", apiPort))
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", apiPort),
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
