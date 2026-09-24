// Package events provides the in-memory, replayable per-commit event bus that
// backs the SSE endpoint. It is the only thing the backend needs to observe a
// run: the judge publishes status transitions, per-case results and the
// terminal `finished` event here.
package events

import (
	"encoding/json"
	"sync"
	"time"
)

// Event names, used as the SSE `event:` field.
const (
	NameStatus      = "status"
	NameCompilation = "compilation"
	NameCaseResult  = "case_result"
	NameArtifact    = "artifact"
	NameFinished    = "finished"
	NameError       = "error"
)

const (
	// maxFrames bounds the replay buffer per commit.
	maxFrames = 1024
	// maxTopicBytes bounds the total retained payload per commit. A frame count
	// alone is not enough: a `case_result` carries the submission's own output, so
	// 1024 of them can pin hundreds of megabytes per commit, and a burst of large
	// frames also stalls subscribers, which are served while the topic is locked.
	maxTopicBytes = 16 << 20
	// subBuffer is the per-subscriber channel capacity; slow subscribers are
	// dropped rather than stalling the run (they recover from the replay buffer).
	subBuffer = 64
	// terminalSendTimeout is how long the terminal frame waits for a stalled
	// subscriber before being dropped with the rest. It is bounded so one slow
	// reader cannot hold up the run it is watching.
	terminalSendTimeout = 2 * time.Second
)

// Frame is one serialized event.
type Frame struct {
	Seq  int64
	Name string
	Data []byte
}

// Hub is a set of per-commit replayable event logs.
type Hub struct {
	mu        sync.Mutex
	topics    map[int64]*topic
	retention time.Duration
}

// New creates a Hub whose finished topics are evicted after retention.
func New(retention time.Duration) *Hub {
	if retention <= 0 {
		retention = 10 * time.Minute
	}

	return &Hub{topics: make(map[int64]*topic), retention: retention}
}

// topic is the replayable event log for one commit.
type topic struct {
	mu     sync.Mutex
	next   int64
	frames []Frame
	bytes  int
	subs   map[chan Frame]struct{}
	done   bool
	evict  *time.Timer
}

// appendFrame adds a frame to the replay buffer, evicting the oldest frames while
// either bound is exceeded. At least one frame is always kept, so a single
// oversized frame cannot push out the terminal event that follows it.
func (t *topic) appendFrame(frame Frame) {
	t.frames = append(t.frames, frame)
	t.bytes += len(frame.Data)

	for len(t.frames) > 1 && (len(t.frames) > maxFrames || t.bytes > maxTopicBytes) {
		t.bytes -= len(t.frames[0].Data)
		// Clear the slot so the evicted payload can be collected: the slice keeps
		// referencing the backing array.
		t.frames[0] = Frame{}
		t.frames = t.frames[1:]
	}
}

// topic returns the topic for a commit, creating it if needed.
func (h *Hub) topic(id int64) *topic {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If the topic is already done and has been evicted, we create a new one.
	// This is safe because the old topic is no longer referenced by any subscribers
	// (they have been closed).
	t, ok := h.topics[id]
	if !ok {
		t = &topic{subs: make(map[chan Frame]struct{})}
		h.topics[id] = t
	}

	return t
}

// seqSetter lets payload structs receive their assigned sequence number.
type seqSetter interface{ setSeq(int64) }

// publish emits a new event to the topic, assigning it a sequence number and notifying subscribers.
func (h *Hub) publish(id int64, name string, payload any) {
	t := h.topic(id)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.done {
		return
	}

	// Increment the sequence number for the new event.
	t.next++

	// If the payload implements seqSetter, set its sequence number.
	if s, ok := payload.(seqSetter); ok {
		s.setSeq(t.next)
	}

	// Marshal the payload to JSON for the event data.
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Create a new Frame with the assigned sequence number, event name, and JSON data.
	frame := Frame{Seq: t.next, Name: name, Data: data}

	// Append the new frame to the replay buffer, evicting the oldest frames if it
	// no longer fits within the frame-count or byte budget.
	t.appendFrame(frame)

	// Notify the subscribers of the new frame. A full channel means a subscriber is
	// behind: the frame is dropped for it, and it recovers from the replay buffer.
	//
	// The terminal frame is the exception: it is the one frame a client cannot
	// reconstruct from anything else, so it waits for room for a bounded time
	// instead of being lost. The wait is bounded because the topic lock is held
	// here and the run must not be held up by a stalled reader.
	terminal := name == NameFinished || name == NameError

	for ch := range t.subs {
		if terminal {
			select {
			case ch <- frame:
			case <-time.After(terminalSendTimeout):
				// Still delivered through the replay buffer, and the topic closes
				// right below, so a client that reconnects reads the result then.
			}
			continue
		}

		select {
		case ch <- frame:
		default:
			// Slow subscriber: drop; it recovers from the replay buffer.
		}
	}

	// Only `finished` terminates the topic: it is the event that carries the
	// authoritative result. `error` carries a message and is always followed by a
	// `finished`, so treating it as terminal would silently drop the terminal
	// status — and with it the real verdict, e.g. turning a timeout into a server
	// error on the backend.
	//
	// An error still arms the eviction timer, so a topic that somehow never gets
	// its `finished` (the process dying between the two publishes is the only way)
	// is reclaimed after the retention period instead of being kept forever.
	if name == NameError && t.evict == nil {
		t.evict = time.AfterFunc(h.retention, func() { h.evictTopic(id, t) })
	}

	// If the event is a terminal event (finished), mark the topic as done,
	// close all subscriber channels, and schedule eviction after the retention period.
	if name == NameFinished {
		t.done = true
		// Close every subscriber so SSE handlers return on their own; a
		// late subscriber gets the backlog from the closed-channel path in
		// Subscribe.
		for ch := range t.subs {
			close(ch)
		}

		// Clear the subscriber map to free memory and prevent further notifications.
		t.subs = make(map[chan Frame]struct{})

		// Schedule eviction of the topic after the retention period.
		t.evict = time.AfterFunc(h.retention, func() { h.evictTopic(id, t) })
	}
}

// evictTopic removes the topic from the hub if it is still the same topic (not replaced by a new one).
func (h *Hub) evictTopic(id int64, t *topic) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.topics[id] == t {
		delete(h.topics, id)
	}
}

// Subscription is a live view of one commit's event log.
type Subscription struct {
	// Backlog holds the frames after the requested `from`, captured atomically
	// with the channel registration so no frame is lost or duplicated.
	Backlog []Frame
	Ch      <-chan Frame
	// Done is true when the topic already finished at subscribe time; the
	// channel is closed and there will be no further frames.
	Done bool

	ch     chan Frame
	cancel func()
}

// Cancel detaches the subscriber.
func (s *Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Subscribe returns the frames after `from` plus a channel of live frames.
func (h *Hub) Subscribe(id, from int64) *Subscription {
	t := h.topic(id)

	t.mu.Lock()
	defer t.mu.Unlock()

	var backlog []Frame

	// Collect frames with sequence numbers greater than `from` to form the backlog.
	for _, f := range t.frames {
		if f.Seq > from {
			backlog = append(backlog, f)
		}
	}

	// If the topic is already done, return a closed channel and mark the subscription as done.
	if t.done {
		ch := make(chan Frame)
		close(ch)
		return &Subscription{Backlog: backlog, Ch: ch, Done: true}
	}

	// Create a buffered channel for live frames and register it in the topic's subscriber map.
	ch := make(chan Frame, subBuffer)
	t.subs[ch] = struct{}{}

	// Define the cancel function to remove the subscriber and close the channel when called.
	sub := &Subscription{Backlog: backlog, Ch: ch}
	sub.cancel = func() {
		t.mu.Lock()
		defer t.mu.Unlock()

		// Remove the subscriber from the topic's subscriber map and close the channel.
		if _, ok := t.subs[ch]; ok {
			delete(t.subs, ch)
			close(ch)
		}
	}

	return sub
}

// --- typed payloads ---------------------------------------------------------

type basePayload struct {
	Type     string `json:"type"`
	CommitID int64  `json:"commit_id"`
	Seq      int64  `json:"seq"`
}

// setSeq sets the sequence number for the payload. It is called by the Hub when publishing an event.
func (b *basePayload) setSeq(s int64) { b.Seq = s }

// StatusPayload is a `status` event (status: compiling|running).
type StatusPayload struct {
	basePayload
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

// CompilationPayload is a `compilation` event.
type CompilationPayload struct {
	basePayload
	Compiled bool      `json:"compiled"`
	Message  string    `json:"message"`
	Error    string    `json:"error"`
	At       time.Time `json:"at"`
}

// CaseResultPayload is a `case_result` event.
type CaseResultPayload struct {
	basePayload
	TestCaseID     int64   `json:"test_case_id"`
	CPUTime        float64 `json:"cpu_time"`
	MemUsage       int64   `json:"mem_usage"`
	Status         string  `json:"status"`
	StatusMessage  string  `json:"status_message"`
	UserOutput     string  `json:"user_output"`
	UserOutputType string  `json:"user_output_type"`
	ErrorMessage   string  `json:"error_message"`
}

// ArtifactPayload is an `artifact` event.
type ArtifactPayload struct {
	basePayload
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// FinishedPayload is the terminal `finished` event.
type FinishedPayload struct {
	basePayload
	Status             string    `json:"status"`
	NumCorrectCases    int       `json:"num_correct_cases"`
	Score              float64   `json:"score"`
	CompilationMessage string    `json:"compilation_message"`
	CompilationError   string    `json:"compilation_error"`
	StartedAt          time.Time `json:"started_at"`
	FinishedAt         time.Time `json:"finished_at"`
}

// ErrorPayload is an `error` event, which also terminates the stream.
type ErrorPayload struct {
	basePayload
	Message string `json:"message"`
}

// Status publishes a `status` event (compiling|running) for a commit.
func (h *Hub) Status(id int64, status string, at time.Time) {
	h.publish(id, NameStatus, &StatusPayload{
		basePayload: basePayload{Type: NameStatus, CommitID: id},
		Status:      status,
		At:          at,
	})
}

// Compilation publishes a `compilation` event for a commit, indicating whether
// compilation succeeded or failed, along with any messages or errors.
func (h *Hub) Compilation(id int64, compiled bool, message, errMsg string, at time.Time) {
	h.publish(id, NameCompilation, &CompilationPayload{
		basePayload: basePayload{Type: NameCompilation, CommitID: id},
		Compiled:    compiled,
		Message:     message,
		Error:       errMsg,
		At:          at,
	})
}

// CaseResult publishes a `case_result` event for a specific test case of a commit,
// including performance metrics and output details.
func (h *Hub) CaseResult(id, testCaseID int64, cpuTime float64, memUsage int64, status, statusMsg, userOutput, outputType, errMsg string) {
	h.publish(id, NameCaseResult, &CaseResultPayload{
		basePayload:    basePayload{Type: NameCaseResult, CommitID: id},
		TestCaseID:     testCaseID,
		CPUTime:        cpuTime,
		MemUsage:       memUsage,
		Status:         status,
		StatusMessage:  statusMsg,
		UserOutput:     userOutput,
		UserOutputType: outputType,
		ErrorMessage:   errMsg,
	})
}

// Artifact publishes an `artifact` event for a commit, indicating the kind of artifact and its URL.
func (h *Hub) Artifact(id int64, kind, url string) {
	h.publish(id, NameArtifact, &ArtifactPayload{
		basePayload: basePayload{Type: NameArtifact, CommitID: id},
		Kind:        kind,
		URL:         url,
	})
}

// Finished publishes the terminal `finished` event for a commit, including the final status, score,
// and compilation messages.
func (h *Hub) Finished(id int64, status string, numCorrect int, score float64, message, errMsg string, startedAt, finishedAt time.Time) {
	h.publish(id, NameFinished, &FinishedPayload{
		basePayload:        basePayload{Type: NameFinished, CommitID: id},
		Status:             status,
		NumCorrectCases:    numCorrect,
		Score:              score,
		CompilationMessage: message,
		CompilationError:   errMsg,
		StartedAt:          startedAt,
		FinishedAt:         finishedAt,
	})
}

// Error publishes an `error` event for a commit, indicating a fatal error that terminates the stream.
func (h *Hub) Error(id int64, message string) {
	h.publish(id, NameError, &ErrorPayload{
		basePayload: basePayload{Type: NameError, CommitID: id},
		Message:     message,
	})
}
