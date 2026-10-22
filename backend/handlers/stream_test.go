package handlers

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

// deadlineRecorder is a ResponseWriter that exposes the deadline hook
// ResponseController looks for. httptest.ResponseRecorder deliberately does not
// implement it, so a stalled client would otherwise be untestable.
type deadlineRecorder struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	deadline time.Time
}

func (w *deadlineRecorder) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *deadlineRecorder) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *deadlineRecorder) WriteHeader(status int) {
	w.status = status
}

func (w *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestDeadlineWriterRefreshesOnEachWrite(t *testing.T) {
	w := &deadlineRecorder{}
	writer := newDeadlineWriter(w)

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	first := w.deadline
	if first.IsZero() {
		t.Fatal("the first write did not set a deadline")
	}
	if w.body.String() != "hello" {
		t.Fatalf("body = %q, want hello", w.body.String())
	}

	time.Sleep(time.Millisecond)
	if _, err := writer.Write([]byte(" world")); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if !w.deadline.After(first) {
		t.Fatalf("deadline was not refreshed: first %s, second %s", first, w.deadline)
	}
}
