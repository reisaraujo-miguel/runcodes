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
	"github.com/go-chi/oauth"
	"github.com/go-chi/traceid"
)

func createRoutes(router *chi.Mux) {
	s := oauth.NewBearerServer(
		"mySecretKey-10101",
		time.Second*120,
		nil,
		nil)

	router.Post("/token", s.UserCredentials)
	router.Post("/auth", s.ClientCredentials)

	router.Post("/api/offerings/create", handlers.CreateOffering)
	router.Get("/api/offerings", handlers.GetOfferings)
}

// Configures traceid, RequestLogger, Recoverer and cors.handler
func configureMiddleware(router *chi.Mux) {
	router.Use(traceid.Middleware)

	router.Use(httplog.RequestLogger(utils.Logger, &httplog.Options{
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
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			httplog.SetAttrs(ctx, slog.String("user", "user1"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTION"},
		AllowedHeaders:   []string{"User-Agent", "Content-Type", "Accept", "Accept-Encoding", "Accept-Language", "Cache-Control", "Connection", "DNT", "Host", "Origin", "Pragma", "Referer"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}
