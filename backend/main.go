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
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"runcodes/handlers"
	"runcodes/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/oauth"
	"github.com/go-chi/traceid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("No .env file found, using environment variables %s\n", err)
	}

	utils.SetupLogger()

	apiPort := os.Getenv("RUNCODES_API_PORT")
	if apiPort == "" {
		utils.Logger.Error("RUNCODES_API_PORT environment variable is not set\n")
		return
	}

	err = utils.InitDB()
	if err != nil {
		return
	}

	r := chi.NewRouter()
	r.Use(traceid.Middleware)

	r.Use(httplog.RequestLogger(utils.Logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             utils.LogFormat,
		RecoverPanics:      true,
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},
		// Log all requests with invalid payload as curl command.
		LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
			if respStatus == 400 || respStatus == 422 {
				req.Header.Del("Authorization")
				return []slog.Attr{slog.String("curl", httplog.CURL(req, reqBody))}
			}
			return nil
		},
	}))

	// Set request log attribute from within middleware.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			httplog.SetAttrs(ctx, slog.String("user", "user1"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTION"},
		AllowedHeaders:   []string{"User-Agent", "Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Connection", "DNT", "Host", "Origin", "Pragma", "Referer"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	s := oauth.NewBearerServer(
		"mySecretKey-10101",
		time.Second*120,
		nil,
		nil)

	r.Post("/token", s.UserCredentials)
	r.Post("/auth", s.ClientCredentials)

	r.Post("/api/offerings/create", handlers.CreateOffering)
	r.Get("/api/offerings", handlers.GetOfferings)

	utils.Logger.Info(fmt.Sprintf("Server is running on port %s\n", apiPort))
	if err := http.ListenAndServe(fmt.Sprintf("localhost:%s", apiPort), r); err != nil {
		utils.Logger.Error("Failed to start server", slog.String("error", err.Error()))
		return
	}
}
