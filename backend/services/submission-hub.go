package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	// subscriberBuffer is how many frames a subscriber may lag behind before
	// frames start being dropped for it (the snapshot covers reconnections).
	subscriberBuffer = 64

	// maxStreamAttempts bounds how many times the hub reconnects to the judge
	// stream (resuming with ?from=<seq>) before giving up.
	maxStreamAttempts = 3
)

// ErrStreamClosed is returned when subscribing to an already finished commit.
var ErrStreamClosed = errors.New("event stream is already closed")

var (
	hubsMu sync.Mutex
	hubs   = make(map[int64]*Hub)
)

/*
Hub owns the single upstream judge SSE connection for one commit and fans its
events out to every connected client (tab). It persists each event before
broadcasting it.
*/
type Hub struct {
	commitID int64

	mu       sync.Mutex
	subs     map[chan []byte]struct{}
	closed   bool
	terminal bool
	warned   bool

	lastSeq int64
	cancel  context.CancelFunc
}

/*
Subscription is a client's view of a commit's event stream. Close must be
called when the client disconnects.
*/
type Subscription struct {
	Events <-chan []byte

	hub  *Hub
	ch   chan []byte
	once sync.Once
}

/*
SubscribeCommit registers a subscriber for a commit, starting the upstream
judge stream if this is the first one.
*/
func SubscribeCommit(commitID int64) (*Subscription, error) {
	hub := getOrCreateHub(commitID)

	ch := make(chan []byte, subscriberBuffer)

	hub.mu.Lock()
	if hub.closed {
		hub.mu.Unlock()
		return nil, ErrStreamClosed
	}
	hub.subs[ch] = struct{}{}
	hub.mu.Unlock()

	return &Subscription{Events: ch, hub: hub, ch: ch}, nil
}

/*
Close unsubscribes the client. The upstream stream is kept alive as long as it
is not terminal so the commit keeps being persisted even with nobody watching.
*/
func (s *Subscription) Close() {
	s.once.Do(func() {
		s.hub.mu.Lock()
		delete(s.hub.subs, s.ch)
		s.hub.mu.Unlock()
	})
}

func getOrCreateHub(commitID int64) *Hub {
	hubsMu.Lock()
	defer hubsMu.Unlock()

	if hub, ok := hubs[commitID]; ok && !hub.isClosed() {
		return hub
	}

	ctx, cancel := context.WithCancel(context.Background())
	hub := &Hub{
		commitID: commitID,
		subs:     make(map[chan []byte]struct{}),
		cancel:   cancel,
	}
	hubs[commitID] = hub

	go hub.run(ctx)

	return hub
}

/*
run reads from the judge stream, persisting and broadcasting every event. It
reconnects (resuming from the last sequence) on transient failures, and gives
up — marking the commit as server_error — if the stream cannot be established.
*/
func (h *Hub) run(ctx context.Context) {
	defer h.finish()

	for attempt := 1; attempt <= maxStreamAttempts; attempt++ {
		err := h.stream(ctx)

		if h.isTerminal() || ctx.Err() != nil {
			return
		}

		if err == nil {
			// Clean EOF without a finished event.
			slog.ErrorContext(ctx,
				"judge stream ended without a finished event",
				slog.Int64("commit_id", h.commitID),
			)
			h.failServerError(ctx, "judge stream ended without a finished event")
			return
		}

		slog.ErrorContext(ctx, "judge event stream failed",
			slog.Int64("commit_id", h.commitID),
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)

		select {
		case <-time.After(time.Duration(attempt) * time.Second):
		case <-ctx.Done():
			return
		}
	}

	slog.ErrorContext(ctx, "giving up on judge event stream",
		slog.Int64("commit_id", h.commitID),
	)
	h.failServerError(ctx, "could not re-establish the judge event stream")
}

/*
stream reads one upstream connection until it ends or the commit finishes.
*/
func (h *Hub) stream(ctx context.Context) error {
	resp, err := openJudgeEvents(ctx, h.commitID, h.lastSeq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return ParseSSE(resp.Body, func(frame SSEFrame) error {
		if len(frame.Data) == 0 {
			return nil
		}

		var ev JudgeEvent
		if err := json.Unmarshal(frame.Data, &ev); err != nil {
			slog.ErrorContext(ctx, "malformed judge event",
				slog.Int64("commit_id", h.commitID),
				slog.String("error", err.Error()),
			)
			return nil
		}

		if ev.Seq > h.lastSeq {
			h.lastSeq = ev.Seq
		}

		if err := h.handleEvent(ctx, &ev, frame.Data); err != nil {
			slog.ErrorContext(ctx, "failed to persist judge event",
				slog.Int64("commit_id", h.commitID),
				slog.String("type", ev.Type),
				slog.String("error", err.Error()),
			)
		}

		return nil
	})
}

/*
handleEvent persists an event, broadcasts it to subscribers and, for the
terminal events, shuts the hub down.
*/
func (h *Hub) handleEvent(ctx context.Context, ev *JudgeEvent, raw []byte) error {
	if ev.Type == "finished" || ev.Type == "error" {
		h.setTerminal()
	}

	persistErr := PersistEvent(ctx, ev)

	h.broadcast(FormatSSEFrame(ev.Type, ev.Seq, raw))

	if ev.Type == "finished" || ev.Type == "error" {
		h.finish()
	}

	return persistErr
}

/*
broadcast sends a frame to every subscriber. Slow subscribers that cannot keep
up have the frame dropped rather than blocking the upstream stream.
*/
func (h *Hub) broadcast(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs {
		select {
		case ch <- frame:
		default:
			if !h.warned {
				h.warned = true
				slog.Warn("dropping judge events for a slow subscriber",
					slog.Int64("commit_id", h.commitID),
				)
			}
		}
	}
}

/*
finish closes every subscriber channel and removes the hub from the registry.
It is safe to call multiple times.
*/
func (h *Hub) finish() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for ch := range h.subs {
		close(ch)
	}
	h.subs = make(map[chan []byte]struct{})
	h.mu.Unlock()

	h.cancel()

	hubsMu.Lock()
	if hubs[h.commitID] == h {
		delete(hubs, h.commitID)
	}
	hubsMu.Unlock()
}

func (h *Hub) setTerminal() {
	h.mu.Lock()
	h.terminal = true
	h.mu.Unlock()
}

func (h *Hub) isTerminal() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.terminal
}

func (h *Hub) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *Hub) failServerError(ctx context.Context, message string) {
	if err := MarkCommitServerError(ctx, h.commitID); err != nil {
		slog.ErrorContext(ctx, "failed to mark commit as server_error",
			slog.Int64("commit_id", h.commitID),
			slog.String("error", err.Error()),
		)
	}
	slog.ErrorContext(ctx, "commit failed",
		slog.Int64("commit_id", h.commitID),
		slog.String("reason", message),
	)
}
