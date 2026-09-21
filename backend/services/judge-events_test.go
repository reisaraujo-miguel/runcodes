package services

import (
	"context"
	"testing"
)

// TestPersistEventGuards covers the paths that must short-circuit before
// touching the database. DB is left nil on purpose: any query would panic.
func TestPersistEventGuards(t *testing.T) {
	tests := []struct {
		name    string
		event   JudgeEvent
		wantErr bool
	}{
		{
			name:  "terminal status is ignored",
			event: JudgeEvent{Type: "status", CommitID: 1, Status: "completed"},
		},
		{
			name:  "artifact is not persisted",
			event: JudgeEvent{Type: "artifact", CommitID: 1, Kind: "logs", URL: "http://x"},
		},
		{
			name:  "unknown type is ignored",
			event: JudgeEvent{Type: "mystery", CommitID: 1},
		},
		{
			name:    "status without a status fails",
			event:   JudgeEvent{Type: "status", CommitID: 1},
			wantErr: true,
		},
		{
			name:    "malformed case_result fails",
			event:   JudgeEvent{Type: "case_result", CommitID: 1, Status: "correct"},
			wantErr: true,
		},
		{
			name: "case_result without status fails",
			event: JudgeEvent{
				Type: "case_result", CommitID: 1, TestCaseID: 2, Status: "",
			},
			wantErr: true,
		},
		{
			name:    "finished without a status fails",
			event:   JudgeEvent{Type: "finished", CommitID: 1},
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
