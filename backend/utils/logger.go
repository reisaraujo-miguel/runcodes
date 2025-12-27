package utils

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

func SetupLogger() {
	isLocalhost := os.Getenv("HOST") == "localhost"
	LogFormat = httplog.SchemaECS.Concise(isLocalhost)

	Logger = slog.New(logHandler(isLocalhost, &slog.HandlerOptions{
		AddSource:   !isLocalhost,
		ReplaceAttr: LogFormat.ReplaceAttr,
	}))

	// Set as a default logger for both slog and log.
	slog.SetDefault(Logger)
	slog.SetLogLoggerLevel(slog.LevelError)
}

func logHandler(isLocalhost bool, handlerOpts *slog.HandlerOptions) slog.Handler {
	if isLocalhost {
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
