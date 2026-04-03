package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"runcodes/handlers"
	"runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/traceid"
)

func createRoutes(router *chi.Mux) {
	// public routes
	router.Group(func(r chi.Router) {
		r.Post("/api/v1/user/signup", handlers.SignUp)
	})

	// protected routes
	router.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(validation.TokenAuth))
		r.Use(jwtauth.Authenticator(validation.TokenAuth))

		r.Post("/api/v1/offerings/create", handlers.CreateOffering)
	})
}

// configureMiddleware configures traceid, RequestLogger, Recoverer and cors.handler
func configureMiddleware(router *chi.Mux) {
	router.Use(traceid.Middleware)

	router.Use(httplog.RequestLogger(Logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             LogFormat,
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},
		LogRequestBody:     isDebugHeaderSet,
		LogResponseBody:    isDebugHeaderSet,
		// Log all requests with invalid payload as curl command.
		LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
			if !isDebugHeaderSet(req) ||
				(respStatus != http.StatusBadRequest && respStatus != http.StatusUnprocessableEntity) {
				return nil
			}
			sanitized := req.Clone(req.Context())
			sanitized.Header.Del("Authorization")
			return []slog.Attr{slog.String("curl", httplog.CURL(sanitized, reqBody))}
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
	return os.Getenv(debugModeEnv) == "true" && r.Header.Get("Debug") == "reveal-body-logs"
}
