package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/runcodes-icmc/judge/internal/model"
)

// QueueStatsSource is the slice of the store the queue gauges need.
type QueueStatsSource interface {
	QueueStats(ctx context.Context) (model.QueueStats, error)
}

/*
RegisterQueueMetrics observes the durable PostgreSQL queue:

	runcodes.judge.queue.depth         {commit}  queued rows
	runcodes.judge.queue.oldest_age    s         age of the oldest queued row

The callback runs on the SDK's collection cycle (10s in the Compose stack), not
on every worker poll, and uses a short timeout so a slow database cannot wedge
metric collection. The returned function unregisters the callback during
shutdown.
*/
func RegisterQueueMetrics(source QueueStatsSource) (func(), error) {
	meter := otel.Meter(instrumentationName)

	depth, err := meter.Int64ObservableGauge(
		"runcodes.judge.queue.depth",
		metric.WithDescription("Commits waiting in the durable queue"),
		metric.WithUnit("{commit}"),
	)
	if err != nil {
		return nil, err
	}

	oldest, err := meter.Float64ObservableGauge(
		"runcodes.judge.queue.oldest_age",
		metric.WithDescription("Age of the oldest queued commit"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	registration, err := meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			stats, err := source.QueueStats(queryCtx)
			if err != nil {
				// Returning the error lets the OTel SDK report it through the
				// configured error handler; stale values are not fabricated.
				return err
			}

			attrs := metric.WithAttributes(attribute.String("queue", "queued"))
			observer.ObserveInt64(depth, stats.Depth, attrs)
			observer.ObserveFloat64(oldest, stats.OldestSeconds, attrs)
			return nil
		},
		depth,
		oldest,
	)
	if err != nil {
		return nil, err
	}

	return func() { _ = registration.Unregister() }, nil
}
