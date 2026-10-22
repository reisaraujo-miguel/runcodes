/*
Package telemetry wires the judge to OpenTelemetry.

It is deliberately self-contained: the exporter endpoint, sampling policy and
batch tuning are read from the standard OpenTelemetry environment variables
(OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_TRACES_SAMPLER, ...), so the same binary can
run with or without a collector and every knob is configurable from Compose or
a Kubernetes manifest.

OpenTelemetry is opt-in. When OTEL_EXPORTER_OTLP_ENDPOINT (or a signal-specific
endpoint) is unset, or OTEL_SDK_DISABLED=true, Setup installs no providers and
the instrumentation stays a no-op. A backend that cannot reach a collector must
still serve requests, so every export is asynchronous and failures are only
reported through the OpenTelemetry error handler (logged at debug level).
*/
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultServiceName is the service.name used when OTEL_SERVICE_NAME is
	// unset. It identifies this process in Prometheus, Loki and Tempo.
	DefaultServiceName = "runcodes-judge"

	// instrumentationName is the OpenTelemetry instrumentation scope. It is
	// stable so dashboards and span metrics can rely on it.
	instrumentationName = "github.com/runcodes-icmc/judge/internal/telemetry"

	defaultEnvironment = "development"
)

var enabled atomic.Bool

// Enabled reports whether Setup installed exporting providers. Instrumentation
// always records request ids, but spans and OTLP export are only live when this
// is true.
func Enabled() bool { return enabled.Load() }

// ServiceName returns the configured service name.
func ServiceName() string {
	if name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); name != "" {
		return name
	}
	return DefaultServiceName
}

// Tracer returns the tracer used by manual instrumentation.
func Tracer() trace.Tracer { return otel.Tracer(instrumentationName) }

/*
Setup configures the global OpenTelemetry providers from the standard
environment variables and starts Go runtime metrics collection.

The returned shutdown function flushes traces, metrics and logs; callers should
invoke it with a bounded context during graceful shutdown. Setup never makes the
application fail: when telemetry is disabled or the exporter cannot be built it
returns an error, and the caller is expected to log it and continue.
*/
func Setup(ctx context.Context) (func(context.Context) error, error) {
	if !exportEnabled() {
		return func(context.Context) error { return nil }, nil
	}

	res, err := buildResource(ctx)
	if err != nil {
		return nil, fmt.Errorf("building OpenTelemetry resource: %w", err)
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP log exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv()),
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(envDuration("OTEL_BSP_SCHEDULE_DELAY", 5*time.Second)),
			sdktrace.WithExportTimeout(envDuration("OTEL_EXPORTER_OTLP_TIMEOUT", 10*time.Second)),
		),
	)

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(envDuration("OTEL_METRIC_EXPORT_INTERVAL", 60*time.Second)),
			sdkmetric.WithTimeout(envDuration("OTEL_EXPORTER_OTLP_TIMEOUT", 10*time.Second)),
		)),
	)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(
			logExporter,
			sdklog.WithExportInterval(envDuration("OTEL_BLRP_SCHEDULE_DELAY", 2*time.Second)),
			sdklog.WithExportTimeout(envDuration("OTEL_EXPORTER_OTLP_TIMEOUT", 10*time.Second)),
		)),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Exporter errors are expected while a collector restarts; debug keeps them
	// available without flooding the application log on every batch.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Debug("opentelemetry error", slog.String("error", err.Error()))
	}))

	// Go runtime metrics (heap, goroutines, GC) are low-cardinality and useful
	// for correlating a latency change with resource pressure. They are
	// best-effort: a failure here must not disable traces or logs.
	_ = runtime.Start(runtime.WithMeterProvider(meterProvider))

	enabled.Store(true)

	return func(ctx context.Context) error {
		enabled.Store(false)
		return errors.Join(
			tracerProvider.Shutdown(ctx),
			meterProvider.Shutdown(ctx),
			loggerProvider.Shutdown(ctx),
		)
	}, nil
}

// exportEnabled reports whether an OTLP endpoint was configured. The Go SDKs
// default to localhost:4318, which would make a plain `go run .` emit connection
// errors forever, so an explicit endpoint is required.
func exportEnabled() bool {
	if truthy(os.Getenv("OTEL_SDK_DISABLED")) {
		return false
	}

	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
	} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}

	return false
}

func buildResource(ctx context.Context) (*resource.Resource, error) {
	version := serviceVersion()
	environment := strings.TrimSpace(os.Getenv("DEPLOYMENT_ENVIRONMENT"))
	if environment == "" {
		environment = defaultEnvironment
	}

	host, _ := os.Hostname()
	instanceID := host
	if instanceID == "" {
		instanceID = "unknown"
	}
	instanceID += "-" + strconv.Itoa(os.Getpid())

	attributes := []attribute.KeyValue{
		semconv.ServiceName(ServiceName()),
		semconv.ServiceNamespace("runcodes"),
		semconv.ServiceInstanceID(instanceID),
		semconv.DeploymentEnvironmentNameKey.String(environment),
	}
	if version != "" {
		attributes = append(attributes, semconv.ServiceVersion(version))
	}

	// Later detectors win: OTEL_RESOURCE_ATTRIBUTES / OTEL_SERVICE_NAME can
	// intentionally override the defaults above.
	return resource.New(ctx,
		resource.WithAttributes(attributes...),
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
	)
}

// samplerFromEnv implements the standard OTEL_TRACES_SAMPLER values that are
// useful for a service. Unknown values fall back to parentbased_always_on so a
// typo never silently drops production traces.
func samplerFromEnv() sdktrace.Sampler {
	ratio := envFloat("OTEL_TRACES_SAMPLER_ARG", 1.0)
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER"))) {
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	case "always_on":
		return sdktrace.AlwaysSample()
	case "parentbased_always_on", "":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

func serviceVersion() string {
	if version := strings.TrimSpace(os.Getenv("OTEL_SERVICE_VERSION")); version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			revision := setting.Value
			if len(revision) > 12 {
				revision = revision[:12]
			}
			return revision
		}
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if milliseconds, err := strconv.Atoi(raw); err == nil {
		if milliseconds > 0 {
			return time.Duration(milliseconds) * time.Millisecond
		}
		return fallback
	}
	if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
