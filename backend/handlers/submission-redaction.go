package handlers

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/runcodes-icmc/runcodes/judge"
)

/*
redactCaseResultFrame removes the program output fields from one judge
`case_result` frame when its test case is hidden. It returns the unchanged frame
for other events or visible cases, and nil when a hidden frame cannot be parsed:
dropping it is safer than forwarding output the author chose not to release.
*/
func redactCaseResultFrame(frame []byte, hidden map[int64]struct{}) []byte {
	if len(hidden) == 0 || !bytes.Contains(frame, []byte("event: case_result")) {
		return frame
	}

	var redacted []byte
	err := judge.ParseSSE(bytes.NewReader(frame), func(f judge.SSEFrame) error {
		if f.Event != "case_result" {
			redacted = frame
			return nil
		}

		var payload map[string]any
		if err := json.Unmarshal(f.Data, &payload); err != nil {
			return err
		}

		caseID, ok := payload["test_case_id"].(float64)
		if !ok {
			redacted = frame
			return nil
		}
		if _, isHidden := hidden[int64(caseID)]; !isHidden {
			redacted = frame
			return nil
		}

		delete(payload, "user_output")
		delete(payload, "error_message")

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		seq, _ := strconv.ParseInt(f.ID, 10, 64)
		redacted = judge.FormatSSEFrame(f.Event, seq, data)
		return nil
	})

	if err != nil || redacted == nil {
		return nil
	}
	return redacted
}
