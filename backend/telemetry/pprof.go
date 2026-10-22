package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"
)

/*
StartPprof exposes Go's pprof handlers on a dedicated internal listener.

Profiling is opt-in: an empty addr returns a nil shutdown function and starts
nothing. The listener is intentionally separate from the application server so
it never becomes reachable through the public API and can be bound to the
container network only. Alloy's pyroscope.scrape reads it for continuous
profiling.

The listener is bound synchronously, so a port conflict is reported at startup
instead of disappearing into a goroutine.
*/
func StartPprof(addr string) (func(context.Context) error, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("pprof server stopped", slog.String("error", err.Error()))
		}
	}()

	slog.Info("pprof server listening", slog.String("addr", addr))
	return server.Shutdown, nil
}
