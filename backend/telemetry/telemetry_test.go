package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSamplerFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		sampler string
		arg     string
		want    string
	}{
		{name: "default", want: "ParentBased{root:AlwaysOn"},
		{name: "always off", sampler: "always_off", want: "AlwaysOff"},
		{name: "parent based ratio", sampler: "parentbased_traceidratio", arg: "0.25", want: "root:TraceIDRatioBased{0.25}"},
		{name: "ratio", sampler: "traceidratio", arg: "0.5", want: "TraceIDRatioBased{0.5}"},
		{name: "out of range ratio is clamped", sampler: "traceidratio", arg: "2", want: "TraceIDRatioBased{1}"},
		{name: "unknown falls back", sampler: "not-a-sampler", want: "ParentBased{root:AlwaysOn"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", test.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", test.arg)

			if got := samplerFromEnv().Description(); !strings.Contains(got, test.want) {
				t.Fatalf("sampler = %q, want it to contain %q", got, test.want)
			}
		})
	}
}

func TestExportEnabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "")
	t.Setenv("OTEL_SDK_DISABLED", "")
	if exportEnabled() {
		t.Fatal("exportEnabled = true without an endpoint")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
	if !exportEnabled() {
		t.Fatal("exportEnabled = false with an endpoint")
	}

	t.Setenv("OTEL_SDK_DISABLED", "true")
	if exportEnabled() {
		t.Fatal("exportEnabled = true with OTEL_SDK_DISABLED")
	}
}

func TestLogHandlerAddsTraceIdentity(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	var output bytes.Buffer
	logger := slog.New(NewLogHandler(DefaultServiceName,
		slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}),
	))

	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	defer span.End()

	logger.InfoContext(ctx, "hello")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON log: %v (%s)", err, output.String())
	}

	if got := entry["trace_id"]; got != span.SpanContext().TraceID().String() {
		t.Errorf("trace_id = %v, want %s", got, span.SpanContext().TraceID())
	}
	if got := entry["span_id"]; got != span.SpanContext().SpanID().String() {
		t.Errorf("span_id = %v, want %s", got, span.SpanContext().SpanID())
	}
}

func TestTraceContextMiddlewareSetsHeader(t *testing.T) {
	handler := TraceContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	traceID := rec.Header().Get("TraceId")
	if len(traceID) != 32 || strings.Trim(traceID, "0123456789abcdef") != "" {
		t.Fatalf("TraceId header = %q, want a 32-character hex trace id", traceID)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHTTPMetricsMiddlewareRecordsRequest(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	router := chi.NewRouter()
	router.Use(HTTPMetricsMiddleware())
	router.Get("/hello/{name}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello/world", nil))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	found := false
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == "http.server.request.duration" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("http.server.request.duration was not recorded")
	}
}
