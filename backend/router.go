package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/handlers"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/traceid"
)

func createRoutes(router *chi.Mux) {
	// unauthenticated liveness probe (Docker HEALTHCHECK / orchestrators)
	router.Get("/healthz", handlers.Health)

	// public routes
	router.Group(func(r chi.Router) {
		r.Post("/api/v1/user/signup", handlers.SignUp)
		r.Post("/api/v1/user/login", handlers.LogIn)
	})

	// protected routes
	router.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(validation.TokenAuth))
		r.Use(jwtauth.Authenticator(validation.TokenAuth))

		r.Get("/api/v1/auth", handlers.GetAuth)
		r.Post("/api/v1/auth/refresh", handlers.RefreshAuth)

		r.Post("/api/v1/submissions", handlers.CreateSubmission)
		r.Get("/api/v1/submissions/{id}/events", handlers.StreamSubmissionEvents)

		// exercise & test-case authoring (access is enforced in the services)
		r.Get("/api/v1/allowed-file-types", handlers.ListAllowedFileTypes)

		r.Post("/api/v1/offerings/{offeringId}/exercises", handlers.CreateExercise)
		r.Get("/api/v1/offerings/{offeringId}/exercises", handlers.ListOfferingExercises)

		r.Get("/api/v1/exercises/{id}", handlers.GetExercise)
		r.Put("/api/v1/exercises/{id}", handlers.UpdateExercise)
		r.Delete("/api/v1/exercises/{id}", handlers.DeleteExercise)

		r.Get("/api/v1/exercises/{id}/test-cases", handlers.ListTestCases)
		r.Post("/api/v1/exercises/{id}/test-cases", handlers.CreateTestCase)
		r.Put("/api/v1/exercises/{id}/test-cases/{caseId}", handlers.UpdateTestCase)
		r.Delete("/api/v1/exercises/{id}/test-cases/{caseId}", handlers.DeleteTestCase)

		r.Get("/api/v1/exercises/{id}/compilation-files", handlers.ListCompilationFiles)
		r.Post("/api/v1/exercises/{id}/compilation-files", handlers.CreateCompilationFile)
		r.Get("/api/v1/exercises/{id}/compilation-files/{fileId}", handlers.GetCompilationFile)
		r.Delete("/api/v1/exercises/{id}/compilation-files/{fileId}", handlers.DeleteCompilationFile)

		r.Get("/api/v1/exercises/{id}/attached-files", handlers.ListAttachedFiles)
		r.Post("/api/v1/exercises/{id}/attached-files", handlers.CreateAttachedFile)
		r.Delete("/api/v1/exercises/{id}/attached-files/{fileId}", handlers.DeleteAttachedFile)

		// professor and admin routes
		r.Group(func(r chi.Router) {
			r.Use(validation.RequireRole("professor", "admin"))

			r.Post("/api/v1/offerings/create", handlers.CreateOffering)
			r.Get("/api/v1/offerings/{id}", handlers.GetOffering)
		})
	})
}

/*
configureMiddleware configures traceid, RequestLogger, Recoverer and cors.handler
*/
func configureMiddleware(router *chi.Mux, cfg *config.Config) {
	router.Use(traceid.Middleware)

	// Bodies (and, on a rejected payload, a replayable curl command) are only
	// logged when the caller asks for them and the server runs in debug mode.
	logBody := func(r *http.Request) bool {
		return cfg.Debug && r.Header.Get("Debug") == "reveal-body-logs"
	}

	router.Use(httplog.RequestLogger(Logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             LogFormat,
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},
		LogRequestBody:     logBody,
		LogResponseBody:    logBody,
		// Log all requests with invalid payload as curl command.
		LogExtraAttrs: func(
			req *http.Request, reqBody string, respStatus int,
		) []slog.Attr {
			if !logBody(req) ||
				(respStatus != http.StatusBadRequest &&
					respStatus != http.StatusUnprocessableEntity) {
				return nil
			}
			sanitized := req.Clone(req.Context())
			sanitized.Header.Del("Authorization")
			return []slog.Attr{slog.String("curl", httplog.CURL(sanitized, reqBody))}
		},
	}))

	router.Use(middleware.Recoverer)

	// The session JWT is delivered via an HttpOnly cookie, so the frontend
	// calls the API with `credentials: "include"`. Cross-origin credentialed
	// requests require `AllowCredentials` and an explicit origin list — the
	// wildcard origin is not allowed by browsers when credentials are used.
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendOrigin},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// The only hop in front of this API is the Caddy container, whose address is
	// dynamic, so count hops instead of enumerating trusted proxy CIDRs. With no
	// trusted_proxies configured, Caddy ignores a client-supplied
	// X-Forwarded-For and writes the address it observed itself, so that entry is
	// the real client rather than a proxy.
	router.Use(middleware.ClientIPFromXFFTrustedProxies(1))

	router.Use(httprate.LimitBy(100, time.Minute, clientIPKey))
}

// clientIPKey returns the canonicalized client IP address for rate limiting
func clientIPKey(r *http.Request) (string, error) {
	return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
}
