package events

import (
	"strings"
	"testing"
	"time"
)

func TestPublishAndSubscribe(t *testing.T) {
	h := New(time.Minute)
	sub := h.Subscribe(1, 0)
	defer sub.Cancel()
	if sub.Done {
		t.Fatal("a fresh topic must not be done")
	}

	h.Status(1, "compiling", time.Now())

	select {
	case frame := <-sub.Ch:
		if frame.Name != NameStatus || frame.Seq != 1 {
			t.Fatalf("unexpected frame: %+v", frame)
		}
		if !strings.Contains(string(frame.Data), `"status":"compiling"`) {
			t.Fatalf("unexpected payload: %s", frame.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("no frame received")
	}
}

func TestBacklogReplay(t *testing.T) {
	h := New(time.Minute)
	h.Status(7, "compiling", time.Now())
	h.Status(7, "running", time.Now())

	sub := h.Subscribe(7, 1) // resume after seq 1
	defer sub.Cancel()
	if len(sub.Backlog) != 1 || sub.Backlog[0].Seq != 2 {
		t.Fatalf("unexpected backlog: %+v", sub.Backlog)
	}
}

// TestErrorDoesNotSwallowFinished pins the terminal-event contract: a run failure
// publishes `error` and then `finished`, and the `finished` event carries the
// authoritative status (a timeout, say). Treating `error` as terminal would drop
// it and leave the backend recording a server error instead.
func TestErrorDoesNotSwallowFinished(t *testing.T) {
	h := New(time.Minute)
	sub := h.Subscribe(4, 0)
	defer sub.Cancel()

	h.Error(4, "timed out waiting for \"run.done\"")
	h.Finished(4, "timeout", 0, 0, "", "", time.Now(), time.Now())

	var names []string
	for frame := range sub.Ch { // closed by the finished event
		names = append(names, frame.Name)
	}

	if len(names) != 2 || names[0] != NameError || names[1] != NameFinished {
		t.Fatalf("frames = %v, want [error finished]", names)
	}

	// The terminal status must be visible to a late subscriber too.
	late := h.Subscribe(4, 0)
	if !late.Done {
		t.Fatal("subscribing after finished should report Done")
	}

	var last Frame
	for _, f := range late.Backlog {
		last = f
	}
	if last.Name != NameFinished || !strings.Contains(string(last.Data), `"status":"timeout"`) {
		t.Fatalf("unexpected terminal frame: %s %s", last.Name, last.Data)
	}
}

func TestFinishedClosesSubscribers(t *testing.T) {
	h := New(time.Minute)
	sub := h.Subscribe(3, 0)

	h.Finished(3, "completed", 1, 100, "", "", time.Now(), time.Now())

	var frames []Frame
	for frame := range sub.Ch { // channel must close after the terminal event
		frames = append(frames, frame)
	}
	if len(frames) != 1 || frames[0].Name != NameFinished {
		t.Fatalf("unexpected frames: %+v", frames)
	}

	late := h.Subscribe(3, 0)
	if !late.Done {
		t.Fatal("subscribing to a finished topic should report Done")
	}
	if len(late.Backlog) != 1 {
		t.Fatalf("late subscriber should get the backlog: %+v", late.Backlog)
	}
}

// TestReplayBufferIsBoundedByBytes pins the memory bound: the frame count alone
// would let a run with large per-case outputs pin hundreds of megabytes.
func TestReplayBufferIsBoundedByBytes(t *testing.T) {
	h := New(time.Minute)

	// Each frame is a quarter of the byte budget; the frame count stays far below
	// maxFrames, so only the byte bound can evict anything.
	payload := strings.Repeat("x", maxTopicBytes/4)
	for i := 0; i < 8; i++ {
		h.CaseResult(1, int64(i), 0, 0, "correct", "", payload, "text", "")
	}

	sub := h.Subscribe(1, 0)
	defer sub.Cancel()

	var total int
	for _, f := range sub.Backlog {
		total += len(f.Data)
	}

	if total > maxTopicBytes {
		t.Fatalf("retained %d bytes, want at most %d", total, maxTopicBytes)
	}
	if len(sub.Backlog) >= 8 {
		t.Fatalf("expected older frames to be evicted, kept %d", len(sub.Backlog))
	}

	// The newest frame must always survive.
	last := sub.Backlog[len(sub.Backlog)-1]
	if !strings.Contains(string(last.Data), `"test_case_id":7`) {
		t.Fatalf("the newest frame was evicted: %s", last.Data)
	}
}

// TestTerminalFrameWaitsForASlowSubscriber pins the one frame a client cannot
// reconstruct: a subscriber that is behind still receives the terminal event (if
// it starts reading within the bounded wait) instead of losing the result.
func TestTerminalFrameWaitsForASlowSubscriber(t *testing.T) {
	h := New(time.Minute)
	sub := h.Subscribe(6, 0)
	defer sub.Cancel()

	// Fill the subscriber's buffer without reading, so the channel is full.
	for i := 0; i < subBuffer; i++ {
		h.Status(6, "running", time.Now())
	}

	// Publish the terminal frame, then start reading: the publish only completes
	// once there is room, so the finished event cannot be dropped for lack of it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Finished(6, "completed", 1, 100, "", "", time.Now(), time.Now())
	}()

	var finished bool
	for frame := range sub.Ch {
		if frame.Name == NameFinished {
			finished = true
		}
	}

	if !finished {
		t.Fatal("the terminal frame was dropped for a subscriber that was reading")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the terminal publish never completed")
	}
}

func TestCaseResultPayload(t *testing.T) {
	h := New(time.Minute)
	sub := h.Subscribe(5, 0)
	defer sub.Cancel()

	h.CaseResult(5, 11, 0.25, 1024, "correct", "", "ok", "text", "")

	frame := <-sub.Ch
	data := string(frame.Data)
	for _, want := range []string{
		`"test_case_id":11`,
		`"cpu_time":0.25`,
		`"mem_usage":1024`,
		`"status":"correct"`,
		`"user_output":"ok"`,
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("missing %q in %s", want, data)
		}
	}
}
