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
	err := godotenv.Load()
	if err != nil {
		slog.Info("No .env file found, using environment variables %s\n", err)
	}

	utils.SetupLogger()

	apiPort := os.Getenv("RUNCODES_API_PORT")
	if apiPort == "" {
		slog.Error("RUNCODES_API_PORT environment variable is not set\n")
		return
	}

	err = utils.InitDB()
	if err != nil {
		slog.Error("Failed to initialize database.")
		return
	}

	r := chi.NewRouter()
	configureMiddleware(r)
	createRoutes(r)

	slog.Info(fmt.Sprintf("Server is running on port %s\n", apiPort))
	if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", apiPort), r); err != nil {
		slog.Error("Server failed", slog.String("error", err.Error()))
		return
	}
}
