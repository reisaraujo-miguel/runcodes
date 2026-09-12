package worker

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runcodes-icmc/judge/internal/config"
	"github.com/runcodes-icmc/judge/internal/model"
)

type fakeStore struct {
	mu    sync.Mutex
	queue []*model.Commit
}

func (f *fakeStore) Claim(context.Context) (*model.Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queue) == 0 {
		return nil, nil
	}
	commit := f.queue[0]
	f.queue = f.queue[1:]
	return commit, nil
}

type fakeProcessor struct {
	mu    sync.Mutex
	seen  map[int64]bool
	delay time.Duration
	want  int
	done  chan struct{}

	active int32
	peak   int32
}

func (p *fakeProcessor) Process(_ context.Context, commit *model.Commit) {
	current := atomic.AddInt32(&p.active, 1)
	for {
		peak := atomic.LoadInt32(&p.peak)
		if current <= peak || atomic.CompareAndSwapInt32(&p.peak, peak, current) {
			break
		}
	}
	time.Sleep(p.delay)

	p.mu.Lock()
	p.seen[commit.ID] = true
	n := len(p.seen)
	p.mu.Unlock()

	atomic.AddInt32(&p.active, -1)
	if n == p.want {
		select {
		case <-p.done:
		default:
			close(p.done)
		}
	}
}

func TestPoolProcessesQueueWithinConcurrency(t *testing.T) {
	const total = 10
	const concurrency = 2

	queue := make([]*model.Commit, 0, total)
	for i := int64(1); i <= total; i++ {
		queue = append(queue, &model.Commit{ID: i})
	}

	store := &fakeStore{queue: queue}
	processor := &fakeProcessor{
		seen:  make(map[int64]bool),
		delay: 15 * time.Millisecond,
		want:  total,
		done:  make(chan struct{}),
	}
	cfg := &config.Config{Concurrency: concurrency, PollInterval: 5 * time.Millisecond}
	pool := New(cfg, store, processor, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pool.Run(ctx)
	pool.Wake()

	select {
	case <-processor.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the queue to drain")
	}
	cancel()
	time.Sleep(20 * time.Millisecond)

	if peak := atomic.LoadInt32(&processor.peak); peak > concurrency {
		t.Fatalf("peak concurrency = %d, want <= %d", peak, concurrency)
	}
	processor.mu.Lock()
	got := len(processor.seen)
	processor.mu.Unlock()
	if got != total {
		t.Fatalf("processed %d commits, want %d", got, total)
	}
}
