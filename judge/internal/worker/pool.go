// Package worker is the durable-queue consumer: it claims `queued` commits from
// Postgres and hands them to the engine, bounded by a configurable concurrency.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

// Store is the slice of the data layer the pool needs.
type Store interface {
	Claim(ctx context.Context) (*model.Commit, error)
}

// Processor runs a single claimed commit.
type Processor interface {
	Process(ctx context.Context, commit *model.Commit)
}

// Pool claims and processes commits.
type Pool struct {
	cfg       *config.Config
	store     Store
	processor Processor
	logger    *slog.Logger

	wake chan struct{}
}

// New builds a Pool.
func New(cfg *config.Config, st Store, processor Processor, logger *slog.Logger) *Pool {
	return &Pool{
		cfg:       cfg,
		store:     st,
		processor: processor,
		logger:    logger,
		wake:      make(chan struct{}, 1),
	}
}

// Wake nudges the poll loop to look for work immediately (used by the API's
// POST /v1/runs/{id}).
func (p *Pool) Wake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Run polls until ctx is cancelled, then waits for in-flight runs.
func (p *Pool) Run(ctx context.Context) {
	// The semaphore limits the number of concurrent runs to cfg.Concurrency.
	// The WaitGroup waits for all in-flight runs to finish before returning.
	sem := make(chan struct{}, p.cfg.Concurrency)
	var wg sync.WaitGroup

	// The ticker triggers a poll every cfg.PollInterval, but the API can also nudge the pool.
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	// The main loop polls for work until the context is cancelled. It waits for either
	// the ticker to tick or the API to nudge it, then calls drain to claim and process commits.
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-p.wake:
		case <-ticker.C:
		}

		p.drain(ctx, sem, &wg)
	}
}

// drain claims commits until the queue is empty or every slot is busy.
func (p *Pool) drain(ctx context.Context, sem chan struct{}, wg *sync.WaitGroup) {
	for {
		// Try to acquire a slot in the semaphore. If all slots are busy, return.
		select {
		case sem <- struct{}{}:
		default:
			return
		}

		// Claim the next commit from the store.
		claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		commit, err := p.store.Claim(claimCtx)
		cancel()
		if err != nil {
			<-sem
			p.logger.Error("could not claim commit", "error", err)
			return
		}

		// If the queue is empty, release the semaphore slot and return.
		if commit == nil {
			<-sem
			return
		}

		// Process the claimed commit in a new goroutine, releasing the semaphore slot when done.
		wg.Go(func() {
			defer func() { <-sem }()
			p.processor.Process(ctx, commit)
		})
	}
}
