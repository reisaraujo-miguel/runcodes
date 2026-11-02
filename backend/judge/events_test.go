package judge

import (
	"context"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/database"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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
		{"plagiarism", true},
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
			name:  "timings without phases are ignored",
			event: Event{Type: "timings", CommitID: 1},
		},
		{
			name: "timings with an empty phase fail",
			event: Event{
				Type: "timings", CommitID: 1,
				Phases: []EventPhase{{Phase: "", DurationMS: 1}},
			},
			wantErr: true,
		},
		{
			name: "timings with a negative duration fail",
			event: Event{
				Type: "timings", CommitID: 1,
				Phases: []EventPhase{{Phase: "run", DurationMS: -1}},
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

func TestPersistTimings(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		db.Close()
	})

	cpu := int64(1000)
	started := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO commit_phase_timings`).
		WithArgs(
			int64(7), "compile", int64(1200), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"monitor", sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = PersistEvent(context.Background(), &Event{
		Type:     "timings",
		CommitID: 7,
		Phases: []EventPhase{{
			Phase:      "compile",
			DurationMS: 1200,
			CPUMS:      &cpu,
			StartedAt:  &started,
			Source:     "monitor",
			Detail:     []byte(`{"pulled":false}`),
		}},
	})
	if err != nil {
		t.Fatalf("PersistEvent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// alertInput is the pure half of persistAlert: the shape of a judge alert event
// as the alerts package records it, including the defaults.
func TestAlertInput(t *testing.T) {
	severity := "critical"
	input, err := alertInput(&Event{
		Type:     "alert",
		CommitID: 12,
		Code:     "startup_timeout",
		Severity: severity,
		Message:  "the container did not start",
		Details:  []byte(`{"language":"Python"}`),
	})
	if err != nil {
		t.Fatalf("alertInput: %v", err)
	}
	if input.Code != "startup_timeout" || input.Severity != "critical" {
		t.Fatalf("unexpected input: %+v", input)
	}
	if input.Source != "judge" {
		t.Fatalf("source = %q, want judge", input.Source)
	}
	if input.CommitID == nil || *input.CommitID != 12 {
		t.Fatalf("commit id = %v, want 12", input.CommitID)
	}
	if string(input.Details) != `{"language":"Python"}` {
		t.Fatalf("details = %s", input.Details)
	}

	// A missing severity is a warning, and a missing commit id is not recorded
	// as commit 0 (which would point at nothing).
	input, err = alertInput(&Event{Code: "disk_full", Message: "the disk is full"})
	if err != nil {
		t.Fatalf("alertInput: %v", err)
	}
	if input.Severity != "warning" || input.CommitID != nil {
		t.Fatalf("defaults not applied: %+v", input)
	}

	for _, ev := range []*Event{
		{Message: "no code"},
		{Code: "no_message"},
	} {
		if _, err := alertInput(ev); err == nil {
			t.Fatalf("alertInput(%+v) accepted an incomplete alert", ev)
		}
	}
}
