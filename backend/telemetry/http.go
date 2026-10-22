package telemetry

import (
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware instruments incoming requests with a server span. It uses the
// chi route pattern as the span name and http.route, so a path with an id does
// not create one time series per id.
func HTTPMiddleware(routes chi.Routes) func(http.Handler) http.Handler {
	return otelchi.Middleware(
		ServiceName(),
		otelchi.WithChiRoutes(routes),
		otelchi.WithPropagators(otel.GetTextMapPropagator()),
	)
}

/*
TraceContextMiddleware guarantees a request has a trace id, writes it to the
TraceId response header and puts it in the request context for the log handler.

otelchi normally creates a valid span before this middleware runs. When tracing
is disabled (no collector) the span context is invalid, so a local-only id is
generated: request correlation keeps working even though nothing is exported.
*/
func TraceContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanContext := trace.SpanContextFromContext(r.Context())
		if !spanContext.IsValid() {
			spanContext = trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: randomTraceID(),
				SpanID:  randomSpanID(),
			})
			r = r.WithContext(trace.ContextWithSpanContext(r.Context(), spanContext))
		}

		w.Header().Set("TraceId", spanContext.TraceID().String())
		w.Header().Set("X-Trace-Sampled", strconv.FormatBool(spanContext.IsSampled()))

		next.ServeHTTP(w, r)
	})
}

type httpMetrics struct {
	duration       metric.Float64Histogram
	activeRequests metric.Int64UpDownCounter
}

/*
HTTPMetricsMiddleware records the standard HTTP server metrics:
http.server.request.duration (histogram) and http.server.active_requests
(up/down counter). They become http_server_request_duration_seconds_* and
http_server_active_requests in Prometheus after the collector's exporter.
*/
func HTTPMetricsMiddleware() func(http.Handler) http.Handler {
	meter := otel.Meter(instrumentationName)
	metrics, err := newHTTPMetrics(meter)
	if err != nil {
		slog.Warn("could not create HTTP metrics instruments; continuing without them",
			slog.String("error", err.Error()),
		)
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			method := r.Method

			metrics.activeRequests.Add(r.Context(), 1, metric.WithAttributes(
				semconv.HTTPRequestMethodKey.String(method),
			))
			defer metrics.activeRequests.Add(r.Context(), -1, metric.WithAttributes(
				semconv.HTTPRequestMethodKey.String(method),
			))

			// httplog already wraps the writer inside this middleware, but the
			// wrapper here is what lets the metrics see the final status code.
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)

			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}

			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}

			attributes := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(method),
				semconv.HTTPRoute(route),
				semconv.HTTPResponseStatusCode(status),
				semconv.URLScheme(scheme),
				semconv.ServerAddress(r.Host),
			}
			metrics.duration.Record(
				r.Context(),
				time.Since(start).Seconds(),
				metric.WithAttributes(attributes...),
			)
		})
	}
}

func newHTTPMetrics(meter metric.Meter) (*httpMetrics, error) {
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)
	if err != nil {
		return nil, err
	}

	active, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP server requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	return &httpMetrics{duration: duration, activeRequests: active}, nil
}

// Transport wraps an outgoing HTTP transport with client spans so trace context
// propagates to the judge and S3. A nil base means http.DefaultTransport.
//
// It wraps unconditionally: package-level HTTP clients are constructed before
// telemetry.Setup runs, and otelhttp is a cheap pass-through when no provider is
// installed.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

func randomTraceID() trace.TraceID {
	var id trace.TraceID
	if _, err := rand.Read(id[:]); err != nil {
		binary.BigEndian.PutUint64(id[:8], uint64(time.Now().UnixNano()))
		binary.BigEndian.PutUint64(id[8:], uint64(time.Now().UnixNano()>>1))
	}
	return id
}

func randomSpanID() trace.SpanID {
	var id trace.SpanID
	if _, err := rand.Read(id[:]); err != nil {
		binary.BigEndian.PutUint64(id[:], uint64(time.Now().UnixNano()))
	}
	return id
}
