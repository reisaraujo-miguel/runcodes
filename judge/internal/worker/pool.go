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
	sem := make(chan struct{}, p.cfg.Concurrency)
	var wg sync.WaitGroup

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

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
		select {
		case sem <- struct{}{}:
		default:
			return // all slots busy
		}

		claimCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		commit, err := p.store.Claim(claimCtx)
		cancel()
		if err != nil {
			<-sem
			p.logger.Error("could not claim commit", "error", err)
			return
		}
		if commit == nil {
			<-sem
			return // queue empty
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			p.processor.Process(ctx, commit)
		}()
	}
}
