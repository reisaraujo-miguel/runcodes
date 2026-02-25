package main

import (
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
)

var auth *oauth.BearerServer = oauth.NewBearerServer(
	os.Getenv("RUNCODES_OAUTH_SECRET"),
	time.Second*120,
	nil,
	nil)

func createRoutes(router *chi.Mux) {
	router.Post("/token", auth.UserCredentials)
	router.Post("/auth", auth.ClientCredentials)

	router.Post("/api/offerings/create", handlers.CreateOffering)
	router.Get("/api/offerings", handlers.GetOfferings)
}

// configureMiddleware configures traceid, RequestLogger, Recoverer and cors.handler
func configureMiddleware(router *chi.Mux) {
	router.Use(traceid.Middleware)

	router.Use(httplog.RequestLogger(utils.Logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             utils.LogFormat,
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},
		LogRequestBody:     isDebugHeaderSet,
		LogResponseBody:    isDebugHeaderSet,
		// Log all requests with invalid payload as curl command.
		LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
			if respStatus == 400 || respStatus == 422 {
				req.Header.Del("Authorization")
				return []slog.Attr{slog.String("curl", httplog.CURL(req, reqBody))}
			}
			return nil
		},
	}))

	router.Use(middleware.Recoverer)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
}

func isDebugHeaderSet(r *http.Request) bool {
	return r.Header.Get("Debug") == "reveal-body-logs"
}
