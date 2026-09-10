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
	// subBuffer is the per-subscriber channel capacity; slow subscribers are
	// dropped rather than stalling the run (they recover from the backlog).
	subBuffer = 64
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

type topic struct {
	mu     sync.Mutex
	next   int64
	frames []Frame
	subs   map[chan Frame]struct{}
	done   bool
	evict  *time.Timer
}

func (h *Hub) topic(id int64) *topic {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.topics[id]
	if !ok {
		t = &topic{subs: make(map[chan Frame]struct{})}
		h.topics[id] = t
	}
	return t
}

// seqSetter lets payload structs receive their assigned sequence number.
type seqSetter interface{ setSeq(int64) }

func (h *Hub) publish(id int64, name string, payload any) {
	t := h.topic(id)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return
	}
	t.next++
	if s, ok := payload.(seqSetter); ok {
		s.setSeq(t.next)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	frame := Frame{Seq: t.next, Name: name, Data: data}
	if len(t.frames) >= maxFrames {
		t.frames = t.frames[1:]
	}
	t.frames = append(t.frames, frame)
	for ch := range t.subs {
		select {
		case ch <- frame:
		default:
			// Slow subscriber: drop; it recovers from the replay buffer.
		}
	}
	if name == NameFinished || name == NameError {
		t.done = true
		t.evict = time.AfterFunc(h.retention, func() { h.evictTopic(id, t) })
	}
}

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
	for _, f := range t.frames {
		if f.Seq > from {
			backlog = append(backlog, f)
		}
	}
	if t.done {
		ch := make(chan Frame)
		close(ch)
		return &Subscription{Backlog: backlog, Ch: ch, Done: true}
	}
	ch := make(chan Frame, subBuffer)
	t.subs[ch] = struct{}{}
	sub := &Subscription{Backlog: backlog, Ch: ch}
	sub.cancel = func() {
		t.mu.Lock()
		defer t.mu.Unlock()
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

func (h *Hub) Status(id int64, status string, at time.Time) {
	h.publish(id, NameStatus, &StatusPayload{
		basePayload: basePayload{Type: NameStatus, CommitID: id},
		Status:      status,
		At:          at,
	})
}

func (h *Hub) Compilation(id int64, compiled bool, message, errMsg string, at time.Time) {
	h.publish(id, NameCompilation, &CompilationPayload{
		basePayload: basePayload{Type: NameCompilation, CommitID: id},
		Compiled:    compiled,
		Message:     message,
		Error:       errMsg,
		At:          at,
	})
}

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

func (h *Hub) Artifact(id int64, kind, url string) {
	h.publish(id, NameArtifact, &ArtifactPayload{
		basePayload: basePayload{Type: NameArtifact, CommitID: id},
		Kind:        kind,
		URL:         url,
	})
}

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

func (h *Hub) Error(id int64, message string) {
	h.publish(id, NameError, &ErrorPayload{
		basePayload: basePayload{Type: NameError, CommitID: id},
		Message:     message,
	})
}
