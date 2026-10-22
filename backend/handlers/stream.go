package handlers

import (
	"io"
	"net/http"
	"time"
)

/*
streamWriteTimeout bounds a single write to a response that streams a file. The
server deliberately has no global WriteTimeout because the submission SSE stream
is long-lived and manages its own deadline, so file downloads must do the same:
without it, a client that stops reading pins the handler and its upstream
connection indefinitely.
*/
const streamWriteTimeout = 60 * time.Second

/*
deadlineWriter refreshes the connection's write deadline on every write. A
single total deadline would cut off a legitimate large download on a slow link;
resetting it per write only expires when the client stops accepting bytes.
*/
type deadlineWriter struct {
	w          io.Writer
	controller *http.ResponseController
}

func newDeadlineWriter(w http.ResponseWriter) io.Writer {
	return &deadlineWriter{
		w:          w,
		controller: http.NewResponseController(w),
	}
}

func (d *deadlineWriter) Write(p []byte) (int, error) {
	// A ResponseWriter that does not support deadlines (for example an
	// httptest.ResponseRecorder) is still served; it just keeps the previous
	// behavior.
	_ = d.controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	return d.w.Write(p)
}
