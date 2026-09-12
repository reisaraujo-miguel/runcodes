package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

/*
JudgeEvent is the union of every event payload streamed by the judge. Only the
fields relevant to a given "type" are populated; the raw JSON is relayed to
clients untouched.
*/
type JudgeEvent struct {
	Type     string `json:"type"`
	CommitID int64  `json:"commit_id"`
	Seq      int64  `json:"seq"`

	// status (compiling|running) and finished
	Status string `json:"status,omitempty"`
	At     string `json:"at,omitempty"`

	// compilation
	Compiled *bool  `json:"compiled,omitempty"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`

	// case_result
	TestCaseID     int64       `json:"test_case_id,omitempty"`
	CPUTime        json.Number `json:"cpu_time,omitempty"`
	MemUsage       *int64      `json:"mem_usage,omitempty"`
	StatusMessage  string      `json:"status_message,omitempty"`
	UserOutput     string      `json:"user_output,omitempty"`
	UserOutputType string      `json:"user_output_type,omitempty"`
	ErrorMessage   string      `json:"error_message,omitempty"`

	// artifact
	Kind string `json:"kind,omitempty"`
	URL  string `json:"url,omitempty"`

	// finished
	NumCorrectCases    *int64      `json:"num_correct_cases,omitempty"`
	Score              json.Number `json:"score,omitempty"`
	CompilationMessage string      `json:"compilation_message,omitempty"`
	CompilationError   string      `json:"compilation_error,omitempty"`
	StartedAt          string      `json:"started_at,omitempty"`
	FinishedAt         string      `json:"finished_at,omitempty"`
}

// terminalStatuses are the commit statuses from which a commit never moves
// away (unless the judge sends its authoritative "finished" event).
var terminalStatuses = map[string]bool{
	"completed":         true,
	"uncompleted":       true,
	"compilation_error": true,
	"server_error":      true,
	"timeout":           true,
}

// nonTerminalGuard is the SQL predicate used to protect terminal commits.
const nonTerminalGuard = `status NOT IN ('completed','uncompleted','compilation_error','server_error','timeout')`

/*
IsTerminalStatus reports whether a commit status is final.
*/
func IsTerminalStatus(status string) bool {
	return terminalStatuses[status]
}

/*
PersistEvent writes a judge event to Postgres. Every update is guarded so a
terminal status is never overwritten by a non-terminal one.
*/
func PersistEvent(ctx context.Context, ev *JudgeEvent) error {
	switch ev.Type {
	case "status":
		if IsTerminalStatus(ev.Status) {
			return nil
		}
		return persistStatus(ctx, ev)
	case "compilation":
		return persistCompilation(ctx, ev)
	case "case_result":
		return persistCaseResult(ctx, ev)
	case "finished":
		return persistFinished(ctx, ev)
	case "artifact":
		// There is no column for artifacts; the URL is only useful transiently.
		slog.InfoContext(ctx, "judge artifact available",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("kind", ev.Kind),
			slog.String("url", ev.URL),
		)
		return nil
	case "error":
		slog.ErrorContext(ctx, "judge reported an error",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("message", ev.Message),
		)
		return MarkCommitServerError(ctx, ev.CommitID)
	default:
		slog.WarnContext(ctx, "ignoring unknown judge event type",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("type", ev.Type),
		)
		return nil
	}
}

func persistStatus(ctx context.Context, ev *JudgeEvent) error {
	if ev.Status == "" {
		return fmt.Errorf("status event without a status")
	}

	if _, err := DB.ExecContext(ctx,
		`UPDATE commits SET status = $1::commit_status_t
		 WHERE id = $2 AND `+nonTerminalGuard,
		ev.Status, ev.CommitID,
	); err != nil {
		return fmt.Errorf("persisting status event: %w", err)
	}

	return nil
}

func persistCompilation(ctx context.Context, ev *JudgeEvent) error {
	compiled := false
	if ev.Compiled != nil {
		compiled = *ev.Compiled
	}

	if _, err := DB.ExecContext(ctx,
		`UPDATE commits
		 SET compiled = $1,
		     compilation_message = $2,
		     compilation_error = $3,
		     compilation_finished = now()
		 WHERE id = $4 AND `+nonTerminalGuard,
		compiled, ev.Message, ev.Error, ev.CommitID,
	); err != nil {
		return fmt.Errorf("persisting compilation event: %w", err)
	}

	return nil
}

func persistCaseResult(ctx context.Context, ev *JudgeEvent) error {
	if ev.TestCaseID == 0 || ev.Status == "" {
		return fmt.Errorf("malformed case_result event")
	}

	cpuTime := "0"
	if ev.CPUTime != "" {
		cpuTime = ev.CPUTime.String()
	}

	var memUsage int64
	if ev.MemUsage != nil {
		memUsage = *ev.MemUsage
	}

	userOutputType := ev.UserOutputType
	if userOutputType == "" {
		userOutputType = "text"
	}

	if _, err := DB.ExecContext(ctx,
		`INSERT INTO commits_exercise_test_cases_results
		     (commit_id, exercise_test_case_id, cpu_time, mem_usage,
		      user_output, user_output_type, status, status_message, error_message)
		 SELECT $1, $2, $3, $4, $5, $6::input_output_type_t,
		        $7::commit_exercise_test_case_status_t, $8, $9
		 WHERE EXISTS (
		     SELECT 1 FROM commits WHERE id = $1 AND `+nonTerminalGuard+`
		 )
		 ON CONFLICT (commit_id, exercise_test_case_id) DO UPDATE SET
		     cpu_time = EXCLUDED.cpu_time,
		     mem_usage = EXCLUDED.mem_usage,
		     user_output = EXCLUDED.user_output,
		     user_output_type = EXCLUDED.user_output_type,
		     status = EXCLUDED.status,
		     status_message = EXCLUDED.status_message,
		     error_message = EXCLUDED.error_message`,
		ev.CommitID, ev.TestCaseID, cpuTime, memUsage, ev.UserOutput,
		userOutputType, ev.Status, ev.StatusMessage, ev.ErrorMessage,
	); err != nil {
		return fmt.Errorf("persisting case_result event: %w", err)
	}

	return nil
}

func persistFinished(ctx context.Context, ev *JudgeEvent) error {
	if ev.Status == "" {
		return fmt.Errorf("finished event without a status")
	}

	var numCorrect int64
	if ev.NumCorrectCases != nil {
		numCorrect = *ev.NumCorrectCases
	}

	score := "0"
	if ev.Score != "" {
		score = ev.Score.String()
	}

	if _, err := DB.ExecContext(ctx,
		`UPDATE commits
		 SET status = $1::commit_status_t,
		     num_correct_cases = $2,
		     score = $3,
		     compilation_message = COALESCE(NULLIF($4, ''), compilation_message),
		     compilation_error = COALESCE(NULLIF($5, ''), compilation_error),
		     compilation_finished = COALESCE(compilation_finished, now())
		 WHERE id = $6`,
		ev.Status, numCorrect, score, ev.CompilationMessage,
		ev.CompilationError, ev.CommitID,
	); err != nil {
		return fmt.Errorf("persisting finished event: %w", err)
	}

	return nil
}

/*
MarkCommitServerError marks a non-terminal commit as failed. Used when the judge
stream ends without a finished event and by the reconciliation sweeper.
*/
func MarkCommitServerError(ctx context.Context, commitID int64) error {
	if _, err := DB.ExecContext(ctx,
		`UPDATE commits
		 SET status = 'server_error',
		     compilation_finished = COALESCE(compilation_finished, now())
		 WHERE id = $1 AND `+nonTerminalGuard,
		commitID,
	); err != nil {
		return fmt.Errorf("marking commit as server_error: %w", err)
	}

	return nil
}
