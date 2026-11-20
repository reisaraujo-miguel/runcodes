package judge

import (
	"context"
	"errors"
	"testing"
)

// TestErrorEventDoesNotTerminateHub pins the division of responsibility: the
// judge's `error` event is a diagnostic, and only `finished` closes the hub.
// Closing on `error` would drop the following `finished` timeout verdict.
func TestErrorEventDoesNotTerminateHub(t *testing.T) {
	hub := &Hub{
		commitID: 1,
		subs:     make(map[chan []byte]struct{}),
	}

	ch := make(chan []byte, 1)
	hub.subs[ch] = struct{}{}

	ev := &Event{Type: "error", CommitID: 1, Message: "run failed"}
	if err := hub.handleEvent(context.Background(), ev, []byte(`{"type":"error"}`)); err != nil {
		t.Fatalf("handleEvent(error) = %v", err)
	}

	if hub.isTerminal() || hub.isClosed() {
		t.Fatal("an error event must not close the hub")
	}

	select {
	case frame := <-ch:
		if len(frame) == 0 {
			t.Fatal("error frame was not broadcast")
		}
	default:
		t.Fatal("error frame was not broadcast")
	}
}

// TestSubscribeCapsTheFanOut makes sure one commit's hub cannot be fanned out
// without bound: each subscriber costs a buffered channel and frame formatting.
func TestSubscribeCapsTheFanOut(t *testing.T) {
	hub := &Hub{
		commitID: 1,
		subs:     make(map[chan []byte]struct{}),
	}

	for i := 0; i < maxHubSubscribers; i++ {
		ch := make(chan []byte, 1)
		if err := hub.subscribe(ch); err != nil {
			t.Fatalf("subscribe #%d: %v", i+1, err)
		}
	}

	if err := hub.subscribe(make(chan []byte, 1)); !errors.Is(err, ErrTooManySubscribers) {
		t.Fatalf("err = %v, want ErrTooManySubscribers", err)
	}
}
