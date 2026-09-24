package judge

import (
	"context"
	"testing"
)

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

// TestPersistEventGuards covers the paths that must short-circuit before
// touching the database. database.DB is left nil on purpose: any query would
// panic.
func TestPersistEventGuards(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name:  "terminal status is ignored",
			event: Event{Type: "status", CommitID: 1, Status: "completed"},
		},
		{
			name:  "artifact is not persisted",
			event: Event{Type: "artifact", CommitID: 1, Kind: "logs", URL: "http://x"},
		},
		{
			name:  "unknown type is ignored",
			event: Event{Type: "mystery", CommitID: 1},
		},
		{
			name:    "status without a status fails",
			event:   Event{Type: "status", CommitID: 1},
			wantErr: true,
		},
		{
			name:    "malformed case_result fails",
			event:   Event{Type: "case_result", CommitID: 1, Status: "correct"},
			wantErr: true,
		},
		{
			name: "case_result without status fails",
			event: Event{
				Type: "case_result", CommitID: 1, TestCaseID: 2, Status: "",
			},
			wantErr: true,
		},
		{
			name:    "finished without a status fails",
			event:   Event{Type: "finished", CommitID: 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PersistEvent(context.Background(), &tt.event)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
