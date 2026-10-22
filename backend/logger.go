package main

import (
	"log/slog"
	"os"

	"github.com/runcodes-icmc/runcodes/telemetry"

	"github.com/go-chi/httplog/v3"
	"github.com/golang-cz/devslog"
)

var (
	Logger    *slog.Logger
	LogFormat *httplog.Schema
)

/*
SetupLogger uses go-chi/httplog as middleware for structured HTTP logging and
wraps the console handler with the OpenTelemetry log handler.

For dev environments (debug mode), uses golang-cz/devslog for pretty printing
logs. In both modes every record also carries trace_id/span_id when a request
has an active span, and is forwarded to the collector when telemetry is enabled
(see the telemetry package). The stdout copy is never dropped: it is what
`docker logs` and the container runtime expose.
*/
func SetupLogger(debug bool) {
	LogFormat = httplog.SchemaECS.Concise(debug)

	Logger = slog.New(telemetry.NewLogHandler(telemetry.ServiceName(),
		logHandler(debug, &slog.HandlerOptions{
			AddSource:   !debug,
			ReplaceAttr: LogFormat.ReplaceAttr,
		}),
	))

	// Set as a default logger for both slog and log.
	slog.SetDefault(Logger)
	slog.SetLogLoggerLevel(slog.LevelError)
}

func logHandler(
	isDevelopmentEnv bool, handlerOpts *slog.HandlerOptions,
) slog.Handler {
	if isDevelopmentEnv {
		// Pretty logs for localhost development.
		return devslog.NewHandler(os.Stdout, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
			HandlerOptions:     handlerOpts,
		})
	}

	// JSON logs for production; trace identity is added by NewLogHandler.
	return slog.NewJSONHandler(os.Stdout, handlerOpts)
}
