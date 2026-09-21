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
