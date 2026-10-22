package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

func caseResultFrame(t *testing.T, caseID int64, output, errMessage string) []byte {
	t.Helper()

	payload := map[string]any{
		"type":          "case_result",
		"test_case_id":  caseID,
		"user_output":   output,
		"error_message": errMessage,
		"status":        "correct",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return judge.FormatSSEFrame("case_result", 3, data)
}

func TestRedactCaseResultFrameHidesReleasedOutput(t *testing.T) {
	hidden := map[int64]struct{}{7: {}}

	frame := caseResultFrame(t, 7, "secret output", "secret stderr")
	got := redactCaseResultFrame(frame, hidden)
	if got == nil {
		t.Fatal("hidden case frame was dropped instead of redacted")
	}
	if strings.Contains(string(got), "secret output") ||
		strings.Contains(string(got), "secret stderr") {
		t.Fatalf("redacted frame still contains hidden output: %s", got)
	}
	if !strings.Contains(string(got), `"test_case_id":7`) {
		t.Fatalf("redacted frame lost the case id: %s", got)
	}
}

func TestRedactCaseResultFrameLeavesVisibleCasesAlone(t *testing.T) {
	frame := caseResultFrame(t, 8, "visible output", "")
	if got := redactCaseResultFrame(frame, map[int64]struct{}{7: {}}); string(got) != string(frame) {
		t.Fatalf("visible case was modified:\n got %s\nwant %s", got, frame)
	}
}

func TestRedactCaseResultFrameLeavesOtherEventsAlone(t *testing.T) {
	frame := judge.FormatSSEFrame("status", 2, []byte(`{"type":"status","status":"running"}`))
	if got := redactCaseResultFrame(frame, map[int64]struct{}{7: {}}); string(got) != string(frame) {
		t.Fatalf("status frame was modified: %s", got)
	}
}

func TestRedactCaseResultFrameDropsMalformedHiddenFrame(t *testing.T) {
	frame := judge.FormatSSEFrame("case_result", 4, []byte(`{"test_case_id":7,"user_output":`))
	if got := redactCaseResultFrame(frame, map[int64]struct{}{7: {}}); got != nil {
		t.Fatalf("malformed hidden frame = %q, want nil", got)
	}
}

func TestRedactSnapshotOutput(t *testing.T) {
	output := "secret"
	stderr := "secret error"
	snapshot := &models.Snapshot{
		Results: []models.CaseResult{
			{ExerciseTestCaseID: 7, UserOutput: &output, ErrorMessage: &stderr},
			{ExerciseTestCaseID: 8, UserOutput: &output},
		},
	}

	services.RedactSnapshotOutput(snapshot, map[int64]struct{}{7: {}})

	if snapshot.Results[0].UserOutput != nil || snapshot.Results[0].ErrorMessage != nil {
		t.Fatalf("hidden result was not redacted: %+v", snapshot.Results[0])
	}
	if snapshot.Results[1].UserOutput == nil {
		t.Fatal("visible result was redacted")
	}
}
