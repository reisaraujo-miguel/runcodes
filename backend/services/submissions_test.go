package services

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

func TestIsTerminalStatus(t *testing.T) {
	tests := []struct {
		status   string
		terminal bool
	}{
		{"pending", false},
		{"queued", false},
		{"compiling", false},
		{"running", false},
		{"plagiarism", false},
		{"completed", true},
		{"uncompleted", true},
		{"compilation_error", true},
		{"server_error", true},
		{"timeout", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := IsTerminalStatus(tt.status); got != tt.terminal {
				t.Errorf("IsTerminalStatus(%q) = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "main.c", "main.c"},
		{"directory is stripped", "../../etc/passwd", "passwd"},
		{"nested path", "a/b/c.py", "c.py"},
		{"windows separator", "a\\b\\c.py", "c.py"},
		{"unsafe characters", "weird name!.c", "weird_name_.c"},
		{"empty", "", "submission"},
		{"dots only", "..", "submission"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilename(tt.input); got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFilenameMatchesExtension(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		extension string
		expected  bool
	}{
		{"simple match", "main.c", "c", true},
		{"case insensitive", "MAIN.C", "c", true},
		{"compound extension", "main.omp.c", "omp.c", true},
		{"prefix collision", "main.omp.c", "mp.c", false},
		{"wrong extension", "main.cpp", "c", false},
		{"no extension", "README", "zip", false},
		{"zip", "project.zip", "zip", true},
		{"empty extension", "main.c", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filenameMatchesExtension(tt.filename, tt.extension); got != tt.expected {
				t.Errorf("filenameMatchesExtension(%q, %q) = %v, want %v",
					tt.filename, tt.extension, got, tt.expected)
			}
		})
	}
}

func TestParseClientIP(t *testing.T) {
	tests := []struct {
		name         string
		forwardedFor string
		remoteAddr   string
		expected     string
	}{
		{"forwarded for first entry", "203.0.113.5, 10.0.0.1", "10.0.0.1:5555", "203.0.113.5"},
		{"remote addr with port", "", "192.168.0.2:1234", "192.168.0.2"},
		{"remote addr without port", "", "192.168.0.2", "192.168.0.2"},
		{"invalid", "not-an-ip", "also-bad", ""},
		{"empty", "", "", ""},
		{"ipv6", "", "[2001:db8::1]:9000", "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseClientIP(tt.forwardedFor, tt.remoteAddr)
			if tt.expected == "" {
				if got != nil {
					t.Errorf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tt.expected)
			}
			if *got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, *got)
			}
		})
	}
}
