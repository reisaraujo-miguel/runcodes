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
	"log"
	"net/http"
	"os"

	"runcodes/errors"
	"runcodes/handlers"
	"runcodes/middleware"
	"runcodes/utils"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		errors.LogError("main.go", "main", "No .env file found, using environment variables", err)
	}

	apiPort := os.Getenv("RUNCODES_API_PORT")
	if apiPort == "" {
		errors.LogFatalError("main.go", "main", "RUNCODES_API_PORT environment variable is not set", nil)
	}

	db := utils.InitDB()
	defer db.Close()

	r := mux.NewRouter()

	// Routes
	protected := r.PathPrefix("/api").Subrouter()
	protected.Use(middleware.AuthMiddleware)
	protected.HandleFunc("/api/offerings/create", handlers.CreateOffering(db)).Methods("POST")
	protected.HandleFunc("/api/offerings", handlers.GetOfferings(db)).Methods("GET")

	handler := middleware.CORSMiddleware(r)

	log.Printf("Server is running on port %s\n", apiPort)
	errors.LogFatalError("main.go", "main", "Server stopped unexpectedly", http.ListenAndServe(apiPort, handler))
}
