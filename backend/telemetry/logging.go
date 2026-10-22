package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

/*
NewLogHandler returns the handler the service should install as its slog
handler. It always writes to stdout (the caller's handler) and, when telemetry
is enabled, also emits the record through the OpenTelemetry logs SDK.

The stdout copy gets trace_id and span_id attributes from the active span so a
log line can be correlated even when it is read with `docker logs`; the OTLP copy
carries the native OpenTelemetry trace context, which Loki stores as structured
metadata and Grafana uses for logs-to-traces navigation.
*/
func NewLogHandler(serviceName string, stdout slog.Handler) slog.Handler {
	handlers := []slog.Handler{&traceContextHandler{next: stdout}}
	if Enabled() {
		handlers = append(handlers, otelslog.NewHandler(serviceName))
	}

	return &multiHandler{
		handlers: handlers,
		gate:     stdout,
	}
}

type multiHandler struct {
	handlers []slog.Handler
	// gate is the stdout handler. It decides whether a record is emitted at
	// all, so the OTLP stream follows the same log level as the console one.
	gate slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.gate.Enabled(ctx, level)
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers, gate: h.gate.WithAttrs(attrs)}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers, gate: h.gate.WithGroup(name)}
}

// traceContextHandler adds the current trace and span ids to the stdout log
// record. OTel's log SDK does this itself for the OTLP copy.
type traceContextHandler struct {
	next slog.Handler
}

func (h *traceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *traceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	return h.next.Handle(ctx, record)
}

func (h *traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceContextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *traceContextHandler) WithGroup(name string) slog.Handler {
	return &traceContextHandler{next: h.next.WithGroup(name)}
}
