// Package api exposes the judge's HTTP surface: readiness probes, the run
// wake-up endpoint and the replayable SSE event stream the backend consumes.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/events"
)

// DBHealth reports whether Postgres is reachable.
type DBHealth interface {
	Ping(ctx context.Context) error
}

// RuntimeHealth reports whether the container runtime is reachable.
type RuntimeHealth interface {
	Ready(ctx context.Context) error
}

// ArtifactLocator resolves a commit's output archive path.
type ArtifactLocator interface {
	ArtifactPath(commitID int64) string
}

// Server wires the HTTP handlers.
type Server struct {
	cfg       *config.Config
	store     DBHealth
	runtime   RuntimeHealth
	artifacts ArtifactLocator
	hub       *events.Hub
	wake      func()
	logger    *slog.Logger
}

// New builds a Server.
func New(
	cfg *config.Config,
	st DBHealth,
	runtime RuntimeHealth,
	artifacts ArtifactLocator,
	hub *events.Hub,
	wake func(),
	logger *slog.Logger,
) *Server {
	return &Server{cfg: cfg, store: st, runtime: runtime, artifacts: artifacts, hub: hub, wake: wake, logger: logger}
}

// Router returns the handler. `/healthz` and `/readyz` are public so container
// orchestrators can probe them; everything under `/v1` requires the shared
// bearer token when one is configured.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)

	r.Group(func(r chi.Router) {
		r.Use(s.auth)
		r.Post("/v1/runs/{id}", s.handleWake)
		r.Get("/v1/runs/{id}/events", s.handleEvents)
		r.Get("/v1/runs/{id}/output", s.handleOutput)
	})
	return r
}

// auth is a middleware that checks the Authorization header for a bearer token
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if no token is configured, allow all requests
		if s.cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := ""

		// check the Authorization header for a bearer token
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			token = h[7:]
		}

		// compare the token in constant time to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AuthToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth responds with a 200 OK if the server is running.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleReady responds with a 200 OK if the server is ready to accept requests.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// check if the database is reachable
	if err := s.store.Ping(ctx); err != nil {
		s.logger.Warn("readiness: database unavailable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}

	// check if the container runtime is reachable
	if err := s.runtime.Ready(ctx); err != nil {
		s.logger.Warn("readiness: podman unavailable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "podman unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleWake is called by the backend to wake up the judge to process a new commit.
func (s *Server) handleWake(w http.ResponseWriter, r *http.Request) {
	// parse the commit ID from the URL
	id, err := commitID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.wake()

	writeJSON(w, http.StatusAccepted, map[string]any{"commit_id": id, "status": "accepted"})
}

// handleEvents streams the replayable event log of one commit as SSE.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id, err := commitID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// check if the ResponseWriter supports flushing, which is required for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	// parse the "from" query parameter, which indicates the starting sequence number for events
	from := int64(0)
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = parsed
		}
	}

	// subscribe to the event hub for the given commit ID and starting sequence number
	sub := s.hub.Subscribe(id, from)
	defer sub.Cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// write any backlog events that were emitted before the subscription was created
	for _, frame := range sub.Backlog {
		writeFrame(w, frame)
	}
	flusher.Flush()

	// if the subscription is already done (e.g., the commit has finished processing), return immediately
	if sub.Done {
		return
	}

	// set up a heartbeat ticker to send periodic pings to keep the connection alive
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// enter a loop to listen for context cancellation, heartbeat ticks, or new events from the subscription channel
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case frame, ok := <-sub.Ch:
			if !ok {
				// the subscription channel is closed, meaning no more events will be sent
				return
			}

			// write the event frame to the response
			writeFrame(w, frame)
			flusher.Flush()
		}
	}
}

// handleOutput serves the output archive of a commit as a downloadable zip file.
func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	id, err := commitID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// check if the output archive exists and open it for reading
	path := s.artifacts.ArtifactPath(id)
	f, err := os.Open(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no output archive"})
		return
	}
	defer f.Close()

	// get the file info to set the correct headers for the response
	info, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stat failed"})
		return
	}

	// set the response headers to indicate a zip file attachment with the commit ID as the filename
	name := fmt.Sprintf("%d.zip", id)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))

	http.ServeContent(w, r, name, info.ModTime(), f)
}

// commitID extracts and validates the commit ID from the URL parameters.
func commitID(r *http.Request) (int64, error) {
	// parse the commit ID from the URL parameter and ensure it's a positive integer
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid commit id")
	}

	return id, nil
}

// writeFrame writes a single SSE frame to the response writer.
func writeFrame(w http.ResponseWriter, frame events.Frame) {
	_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", frame.Seq, frame.Name, frame.Data)
}

// writeJSON writes a JSON response with the given status code and body.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
