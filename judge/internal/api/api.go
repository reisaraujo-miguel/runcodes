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
	"github.com/runcodes-icmc/judge/internal/engine"
	"github.com/runcodes-icmc/judge/internal/events"
	"github.com/runcodes-icmc/judge/internal/podman"
	"github.com/runcodes-icmc/judge/internal/store"
)

// Server wires the HTTP handlers.
type Server struct {
	cfg    *config.Config
	store  *store.Store
	podman *podman.Client
	hub    *events.Hub
	engine *engine.Engine
	wake   func()
	logger *slog.Logger
}

// New builds a Server.
func New(
	cfg *config.Config,
	st *store.Store,
	pc *podman.Client,
	hub *events.Hub,
	eng *engine.Engine,
	wake func(),
	logger *slog.Logger,
) *Server {
	return &Server{cfg: cfg, store: st, podman: pc, hub: hub, engine: eng, wake: wake, logger: logger}
}

// Router returns the handler. `/healthz` and `/readyz` are public so container
// orchestrators can probe them; everything under `/v1` requires the shared
// bearer token when one is configured.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

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

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := ""
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			token = h[7:]
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AuthToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		s.logger.Warn("readiness: database unavailable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	if err := s.podman.Ready(ctx); err != nil {
		s.logger.Warn("readiness: podman unavailable", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "podman unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWake(w http.ResponseWriter, r *http.Request) {
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
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	from := int64(0)
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = parsed
		}
	}

	sub := s.hub.Subscribe(id, from)
	defer sub.Cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	for _, frame := range sub.Backlog {
		writeFrame(w, frame)
	}
	flusher.Flush()
	if sub.Done {
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

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
				return
			}
			writeFrame(w, frame)
			flusher.Flush()
		}
	}
}

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	id, err := commitID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	path := s.engine.ArtifactPath(id)
	f, err := os.Open(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no output archive"})
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stat failed"})
		return
	}
	name := fmt.Sprintf("%d.zip", id)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeContent(w, r, name, info.ModTime(), f)
}

func commitID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid commit id")
	}
	return id, nil
}

func writeFrame(w http.ResponseWriter, frame events.Frame) {
	_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", frame.Seq, frame.Name, frame.Data)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
