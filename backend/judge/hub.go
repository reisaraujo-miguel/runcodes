package judge

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

const (
	// subscriberBuffer is how many frames a subscriber may lag behind before
	// frames start being dropped for it (the snapshot covers reconnections).
	subscriberBuffer = 64

	// maxStreamAttempts bounds how many times the hub reconnects to the judge
	// stream (resuming with ?from=<seq>) before giving up.
	maxStreamAttempts = 3

	// terminalPersistAttempts bounds the in-line retries when persisting a
	// terminal event, so a short database blip does not drop the authoritative
	// result and leave the commit non-terminal until reconciliation.
	terminalPersistAttempts = 3
)

// ErrStreamClosed is returned when subscribing to an already finished commit.
var ErrStreamClosed = errors.New("event stream is already closed")

// hubLifetimeMargin is added to the stale timeout to bound how long a hub keeps
// its upstream judge stream open.
const hubLifetimeMargin = 5 * time.Minute

var (
	hubsMu sync.Mutex
	hubs   = make(map[int64]*Hub)
)

/*
hubLifetime bounds one hub's upstream connection. It is deliberately longer than
the stale timeout, at which the sweeper settles a stuck commit: tearing the stream
down earlier would abandon a run the judge might still finish.
*/
func hubLifetime() time.Duration {
	return config.Get().Judge.StaleTimeout + hubLifetimeMargin
}

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
Consume starts (or joins) the background consumption of a commit's judge event
stream, so events are persisted even while nobody is watching. It returns
immediately; the stream is torn down when the commit reaches a terminal status.
*/
func Consume(commitID int64) {
	getOrCreateHub(commitID)
}

/*
Subscribe registers a subscriber for a commit, starting the upstream judge
stream if this is the first one.
*/
func Subscribe(commitID int64) (*Subscription, error) {
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

	// The hub owns a connection to the judge (and a goroutine); a deadline keeps a
	// half-open stream from pinning both for the life of the process, while still
	// outliving the window in which the run can legitimately finish.
	ctx, cancel := context.WithTimeout(context.Background(), hubLifetime())
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
CloseHub tears down a commit's hub if one exists. The reconciliation sweeper uses
it once a commit has been settled in the database, so the hub stops holding a
judge stream (and a goroutine) for a run that can no longer report anything.
*/
func CloseHub(commitID int64) {
	hubsMu.Lock()
	hub, ok := hubs[commitID]
	hubsMu.Unlock()

	if ok {
		hub.finish()
	}
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
	resp, err := openEvents(ctx, h.commitID, h.lastSeq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return ParseSSE(resp.Body, func(frame SSEFrame) error {
		if len(frame.Data) == 0 {
			return nil
		}

		var ev Event
		if err := json.Unmarshal(frame.Data, &ev); err != nil {
			slog.ErrorContext(ctx, "malformed judge event",
				slog.Int64("commit_id", h.commitID),
				slog.String("error", err.Error()),
			)
			return nil
		}

		if err := h.handleEvent(ctx, &ev, frame.Data); err != nil {
			// Any persistence failure is fatal to this connection: report it so
			// run reconnects from lastSeq and the judge replays the event, instead
			// of advancing the cursor past an event that was never durably stored.
			slog.ErrorContext(ctx, "failed to persist judge event",
				slog.Int64("commit_id", h.commitID),
				slog.String("type", ev.Type),
				slog.String("error", err.Error()),
			)
			return err
		}

		// Advance the resume cursor only after the event was persisted, so a
		// failed event is replayed on reconnect.
		if ev.Seq > h.lastSeq {
			h.lastSeq = ev.Seq
		}

		return nil
	})
}

/*
handleEvent persists an event, broadcasts it to subscribers and, for the
terminal events, shuts the hub down. Any persistence failure is returned so the
caller reconnects from the last persisted sequence and the judge replays the
event; the writes are idempotent upserts, so a replayed event is safe. Terminal
events are retried in-line before the failure is returned.
*/
func (h *Hub) handleEvent(ctx context.Context, ev *Event, raw []byte) error {
	if ev.Type != "finished" && ev.Type != "error" {
		// A non-terminal event (status, compilation, case_result) is now also
		// persisted before broadcast and must not be lost: return the failure
		// so the cursor is not advanced past an unpersisted event.
		if err := PersistEvent(ctx, ev); err != nil {
			return err
		}

		h.broadcast(FormatSSEFrame(ev.Type, ev.Seq, raw))
		return nil
	}

	// Terminal events carry the authoritative result: persist them before closing
	// the hub, so a transient database failure cannot lose the result and leave
	// the commit non-terminal until reconciliation.
	if err := persistTerminalEvent(ctx, ev); err != nil {
		return err
	}

	h.setTerminal()
	h.broadcast(FormatSSEFrame(ev.Type, ev.Seq, raw))
	h.finish()
	return nil
}

/*
persistTerminalEvent persists a terminal judge event, retrying a few times so a
short database blip does not drop the authoritative result. When it ultimately
fails, the caller leaves the hub open so the event is replayed on reconnect.
*/
func persistTerminalEvent(ctx context.Context, ev *Event) error {
	var err error
	for attempt := 1; attempt <= terminalPersistAttempts; attempt++ {
		if err = PersistEvent(ctx, ev); err == nil {
			return nil
		}

		slog.WarnContext(ctx, "retrying terminal judge event persistence",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("type", ev.Type),
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()),
		)

		select {
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		case <-ctx.Done():
			return err
		}
	}

	return err
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
