package judge

import (
	"strings"
	"testing"
)

func TestParseSSE(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SSEFrame
	}{
		{
			name:  "single frame",
			input: "event: status\nid: 1\ndata: {\"x\":1}\n\n",
			expected: []SSEFrame{
				{Event: "status", ID: "1", Data: []byte(`{"x":1}`)},
			},
		},
		{
			name:  "comments and heartbeats are ignored",
			input: ": ping\n\nevent: status\nid: 2\ndata: {}\n\n: ping\n\n",
			expected: []SSEFrame{
				{Event: "status", ID: "2", Data: []byte(`{}`)},
			},
		},
		{
			name:  "multi-line data is joined with newlines",
			input: "event: case_result\nid: 3\ndata: line one\ndata: line two\n\n",
			expected: []SSEFrame{
				{Event: "case_result", ID: "3", Data: []byte("line one\nline two")},
			},
		},
		{
			name:  "multiple frames",
			input: "event: status\ndata: a\n\nevent: finished\ndata: b\n\n",
			expected: []SSEFrame{
				{Event: "status", ID: "", Data: []byte("a")},
				{Event: "finished", ID: "", Data: []byte("b")},
			},
		},
		{
			name:  "trailing frame without blank line",
			input: "event: error\ndata: boom\n",
			expected: []SSEFrame{
				{Event: "error", ID: "", Data: []byte("boom")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []SSEFrame
			err := ParseSSE(strings.NewReader(tt.input), func(frame SSEFrame) error {
				got = append(got, frame)
				return nil
			})
			if err != nil {
				t.Fatalf("ParseSSE returned error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d frames, got %d: %+v", len(tt.expected), len(got), got)
			}

			for i := range tt.expected {
				if got[i].Event != tt.expected[i].Event {
					t.Errorf("frame %d: expected event %q, got %q", i, tt.expected[i].Event, got[i].Event)
				}
				if got[i].ID != tt.expected[i].ID {
					t.Errorf("frame %d: expected id %q, got %q", i, tt.expected[i].ID, got[i].ID)
				}
				if string(got[i].Data) != string(tt.expected[i].Data) {
					t.Errorf("frame %d: expected data %q, got %q", i, tt.expected[i].Data, got[i].Data)
				}
			}
		})
	}
}

func TestFormatSSEFrame(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		id       int64
		data     []byte
		expected string
	}{
		{
			name:     "with sequence",
			event:    "status",
			id:       7,
			data:     []byte(`{"type":"status"}`),
			expected: "event: status\nid: 7\ndata: {\"type\":\"status\"}\n\n",
		},
		{
			name:     "without sequence",
			event:    "snapshot",
			id:       0,
			data:     []byte(`{"type":"snapshot"}`),
			expected: "event: snapshot\ndata: {\"type\":\"snapshot\"}\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(FormatSSEFrame(tt.event, tt.id, tt.data))
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
