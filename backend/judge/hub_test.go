package judge

import (
	"context"
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
