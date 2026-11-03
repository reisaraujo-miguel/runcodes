package engine

import (
	"testing"

	"github.com/runcodes-icmc/judge/internal/model"
)

func TestCaseDoneID(t *testing.T) {
	tests := []struct {
		line  string
		nonce string
		want  int64
		ok    bool
	}{
		{"case.done 7 abc123", "abc123", 7, true},
		{"case.done 12 abc123", "abc123", 12, true},
		{"case.done 7", "abc123", 0, false},
		{"case.done 7 other", "abc123", 0, false},
		{"case.done abc abc123", "abc123", 0, false},
		{"run.done abc123", "abc123", 0, false},
		{"case.done 7 abc123", "", 0, false},
	}

	for _, tt := range tests {
		got, ok := caseDoneID(tt.line, tt.nonce)
		if got != tt.want || ok != tt.ok {
			t.Errorf("caseDoneID(%q, %q) = %d, %v; want %d, %v",
				tt.line, tt.nonce, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCaseStreamSkipsGradedCases(t *testing.T) {
	stream := newCaseStream()
	stream.store(model.CaseResult{TestCaseID: 4, Status: model.CaseCorrect})

	if _, ok := stream.lookup(4); !ok {
		t.Fatal("stored result not found")
	}
	if _, ok := stream.lookup(5); ok {
		t.Fatal("unknown case reported as stored")
	}
}
