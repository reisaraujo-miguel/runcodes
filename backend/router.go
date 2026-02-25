package main

import (
	"log/slog"
	"net/http"
	"time"

	"runcodes/handlers"
	"runcodes/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/traceid"
)

func createRoutes(router *chi.Mux) {
	router.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(utils.TokenAuth))
		r.Use(jwtauth.Authenticator(utils.TokenAuth))

		router.Post("/api/offerings/create", handlers.CreateOffering)
		router.Get("/api/offerings", handlers.GetOfferings)
	})
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
				sanitized := req.Clone(req.Context())
				sanitized.Header.Del("Authorization")
				return []slog.Attr{slog.String("curl", httplog.CURL(sanitized, reqBody))}
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

	router.Use(httprate.LimitByIP(100, 1*time.Minute))
}

// isDebugHeaderSet returns if the debug header is set on the request
func isDebugHeaderSet(r *http.Request) bool {
	return r.Header.Get("Debug") == "reveal-body-logs"
}
