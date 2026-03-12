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
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"runcodes/utils"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	debugMode := flag.Bool("debug", false, "Sets the server to development mode")
	flag.Parse()

	if *debugMode {
		os.Setenv("DEBUG_MODE", "true")
	}

	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using environment variables", slog.String("error", err.Error()))
	}

	utils.SetupLogger()

	apiPort := os.Getenv("RUNCODES_API_PORT")
	if apiPort == "" {
		slog.Error("RUNCODES_API_PORT environment variable is not set")
		os.Exit(1)
	}

	if err := utils.InitDB(); err != nil {
		slog.Error("Failed to initialize database")
		os.Exit(1)
	}

	if err := utils.SetupJWT(); err != nil {
		slog.Error("Failed to setup JWT", slog.String("error", err.Error()))
		os.Exit(1)
	}

	r := chi.NewRouter()
	configureMiddleware(r)
	createRoutes(r)

	if *debugMode {
		slog.Info("Server is running in debug mode", slog.String("port", apiPort))
	} else {
		slog.Info("Server is running", slog.String("port", apiPort))
	}
	if err := http.ListenAndServe(fmt.Sprintf(":%s", apiPort), r); err != nil {
		slog.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
