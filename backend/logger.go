package main

import (
	"log/slog"
	"os"

	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
	_ "github.com/lib/pq"
)

var (
	Logger    *slog.Logger
	LogFormat *httplog.Schema
)

/*
SetupLogger uses go-chi/httplog and go-chi/traceid as middleware for
structured HTTP logging.

For dev environments (debug mode), uses golang-cz/devslog for pretty printing
logs.
*/
func SetupLogger(debug bool) {
	LogFormat = httplog.SchemaECS.Concise(debug)

	Logger = slog.New(logHandler(debug, &slog.HandlerOptions{
		AddSource:   !debug,
		ReplaceAttr: LogFormat.ReplaceAttr,
	}))

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

	// JSON logs for production with "traceId".
	return traceid.LogHandler(
		slog.NewJSONHandler(os.Stdout, handlerOpts),
	)
}
