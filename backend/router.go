package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/handlers"
	"github.com/runcodes-icmc/runcodes/telemetry"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/httprate"
	"github.com/go-chi/jwtauth/v5"
)

func createRoutes(router *chi.Mux) {
	// unauthenticated liveness probe (Docker HEALTHCHECK / orchestrators)
	router.Get("/healthz", handlers.Health)

	// readiness probe: the database is required, the judge is reported but does
	// not make the API unready (submissions fail, the rest of the platform works)
	router.Get("/readyz", handlers.Ready)

	// public routes
	router.Group(func(r chi.Router) {
		r.Post("/api/v1/user/signup", handlers.SignUp)
		r.Post("/api/v1/user/login", handlers.LogIn)

		// Account email flows are public: the links arrive by email and may be
		// opened in a browser that has no session. These routes are limited
		// more tightly than the API as a whole because they send mail and
		// accept a secret from an unauthenticated caller.
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitBy(10, time.Minute, clientIPKey))

			r.Post("/api/v1/user/verify-email", handlers.VerifyEmail)
			r.Post("/api/v1/user/resend-verification", handlers.ResendVerification)

			r.Post("/api/v1/user/forgot-password", handlers.ForgotPassword)
			r.Post("/api/v1/user/reset-password", handlers.ResetPassword)
		})

		// The contact information the login page shows. It is public by
		// design: a visitor must be able to read it before signing in.
		r.Get("/api/v1/settings/public", handlers.GetPublicSettings)
	})

	// protected routes
	router.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(validation.TokenAuth))
		r.Use(jwtauth.Authenticator(validation.TokenAuth))

		// Signing out stays available to an account that is no longer verified:
		// it is how the browser drops a session cookie that cannot be used.
		r.Post("/api/v1/user/logout", handlers.LogOut)

		// Everything else requires a confirmed email. Login enforces the same
		// rule for new sessions; this covers a session whose account was
		// unconfirmed after it was issued.
		r.Group(func(r chi.Router) {
			r.Use(handlers.RequireVerifiedEmail)

			r.Get("/api/v1/auth", handlers.GetAuth)
			r.Post("/api/v1/auth/refresh", handlers.RefreshAuth)

			// the caller's own account
			r.Get("/api/v1/user/profile", handlers.GetProfile)
			r.Put("/api/v1/user/profile", handlers.UpdateProfile)
			r.Put("/api/v1/user/password", handlers.ChangePassword)

			// the caller's own classes and open exercises (home page)
			r.Get("/api/v1/user/offerings", handlers.ListMyOfferings)
			r.Get("/api/v1/user/offerings/{id}", handlers.GetMyOffering)
			r.Get("/api/v1/user/exercises", handlers.ListMyOpenExercises)

			r.Post("/api/v1/offerings/enroll", handlers.Enroll)
			r.Delete("/api/v1/offerings/{id}/enrollment", handlers.Unenroll)

			r.Post("/api/v1/submissions", handlers.CreateSubmission)
			r.Get("/api/v1/submissions/{id}/events", handlers.StreamSubmissionEvents)
			r.Get("/api/v1/submissions/{id}/source", handlers.GetSubmissionSource)
			r.Get("/api/v1/submissions/{id}/output", handlers.GetSubmissionOutput)
			r.Get("/api/v1/exercises/{id}/submissions", handlers.ListSubmissionHistory)

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
			r.Get("/api/v1/exercises/{id}/attached-files/{fileId}", handlers.GetAttachedFile)
			r.Post("/api/v1/exercises/{id}/attached-files", handlers.CreateAttachedFile)
			r.Delete("/api/v1/exercises/{id}/attached-files/{fileId}", handlers.DeleteAttachedFile)

			// professor and admin routes: own classes and the people in them
			r.Group(func(r chi.Router) {
				r.Use(validation.RequireRole("professor", "admin"))

				r.Get("/api/v1/offerings", handlers.ListOfferings)
				r.Post("/api/v1/offerings/create", handlers.CreateOffering)
				r.Get("/api/v1/offerings/{id}", handlers.GetOffering)
				r.Put("/api/v1/offerings/{id}", handlers.UpdateOffering)
				r.Delete("/api/v1/offerings/{id}", handlers.DeleteOffering)

				r.Get("/api/v1/offerings/{id}/members", handlers.ListOfferingMembers)
				r.Post("/api/v1/offerings/{id}/members", handlers.AddOfferingMember)
				r.Put("/api/v1/offerings/{id}/members/{userId}", handlers.UpdateOfferingMember)
				r.Delete("/api/v1/offerings/{id}/members/{userId}", handlers.RemoveOfferingMember)
			})

			// admin panel
			r.Group(func(r chi.Router) {
				r.Use(validation.RequireRole("admin"))

				r.Get("/api/v1/admin/users", handlers.AdminListUsers)
				r.Put("/api/v1/admin/users/{id}", handlers.AdminUpdateUser)
				r.Delete("/api/v1/admin/users/{id}", handlers.AdminDeleteUser)

				r.Get("/api/v1/admin/offerings", handlers.AdminListOfferings)
				r.Put("/api/v1/admin/offerings/{id}", handlers.AdminUpdateOffering)
				r.Delete("/api/v1/admin/offerings/{id}", handlers.AdminDeleteOffering)
				r.Get("/api/v1/admin/offerings/{id}/members", handlers.AdminListOfferingMembers)

				r.Get("/api/v1/admin/settings", handlers.GetSettings)
				r.Put("/api/v1/admin/settings", handlers.UpdateSettings)
			})
		})
	})
}

/*
configureMiddleware configures OpenTelemetry, RequestLogger, Recoverer and
cors.handler.

The OpenTelemetry middleware runs first so the span exists before the request
logger and the application log records run; telemetry.TraceContextMiddleware
then guarantees every request (including one with tracing disabled) has a
TraceId response header and a trace context for log correlation.
*/
func configureMiddleware(router *chi.Mux, cfg *config.Config) {
	router.Use(telemetry.HTTPMiddleware(router))
	router.Use(telemetry.TraceContextMiddleware)
	router.Use(telemetry.HTTPMetricsMiddleware())

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

	// Request bodies are small (a profile, a submission's exercise id, admin
	// filters). Multipart upload endpoints already install their own, larger
	// MaxBytesReader, so their bodies are skipped here. Without this, the public
	// signup/login endpoints read an unbounded body into memory.
	router.Use(limitRequestBody)

	// The session JWT is delivered via an HttpOnly cookie, so the frontend
	// calls the API with `credentials: "include"`. Cross-origin credentialed
	// requests require `AllowCredentials` and an explicit origin list — the
	// wildcard origin is not allowed by browsers when credentials are used.
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendOrigin},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Traceparent", "Tracestate"},
		ExposedHeaders:   []string{"Link", "TraceId", "X-Trace-Sampled"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// The only hop in front of this API is the Caddy container, whose address is
	// dynamic, so count hops instead of enumerating trusted proxy CIDRs. Caddy
	// appends the address it observed to any client-supplied X-Forwarded-For;
	// counting one trusted hop makes the middleware read that appended entry, so
	// a spoofed header does not become the client IP. Handlers that need the
	// address must use GetClientIP, not the raw header.
	router.Use(middleware.ClientIPFromXFFTrustedProxies(1))

	router.Use(httprate.LimitBy(100, time.Minute, clientIPKey))
}

// maxRequestBodyBytes bounds a non-multipart request body. It is deliberately
// generous for an API whose largest JSON payload is a list of ids, while still
// far below the multipart upload limits the authoring endpoints enforce.
const maxRequestBodyBytes = 1 << 20

// requestBodyReadTimeout bounds how long a client may spend sending a body. It is
// applied per request rather than as a server-wide ReadTimeout, which would also
// cover long-lived SSE requests that have no body to bound.
const requestBodyReadTimeout = 30 * time.Second

// multipartBodyReadTimeout is the longer budget for file uploads. It still
// bounds a slow upload, which would otherwise pin the handler and its temporary
// storage indefinitely.
const multipartBodyReadTimeout = 5 * time.Minute

/*
limitRequestBody caps non-multipart request bodies with http.MaxBytesReader and
gives body-carrying methods a read deadline. The cap is not keyed on the
Content-Type header: that header is client-controlled, so an attacker could omit
it and bypass a JSON-only limit. Multipart requests are not byte-capped here
because their handlers install a limit that matches the file size they accept.
*/
func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		multipart := strings.HasPrefix(
			r.Header.Get("Content-Type"), "multipart/form-data",
		)
		if !multipart {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}

		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			timeout := requestBodyReadTimeout
			if multipart {
				timeout = multipartBodyReadTimeout
			}

			controller := http.NewResponseController(w)
			if err := controller.SetReadDeadline(time.Now().Add(timeout)); err == nil {
				// Do not leave the deadline on a keep-alive connection: the next
				// request may be a multipart upload or a long-lived SSE stream.
				defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
			}
		}

		next.ServeHTTP(w, r)
	})
}

// clientIPKey returns the canonicalized client IP address for rate limiting
func clientIPKey(r *http.Request) (string, error) {
	return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
}
