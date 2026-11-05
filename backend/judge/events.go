package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/runcodes-icmc/runcodes/alerts"
	"github.com/runcodes-icmc/runcodes/database"
)

// EventPhase is one measured phase in a `timings` event.
type EventPhase struct {
	Phase        string          `json:"phase"`
	DurationMS   int64           `json:"duration_ms"`
	CPUMS        *int64          `json:"cpu_ms,omitempty"`
	MemPeakBytes *int64          `json:"mem_peak_bytes,omitempty"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	FinishedAt   *time.Time      `json:"finished_at,omitempty"`
	Source       string          `json:"source,omitempty"`
	Detail       json.RawMessage `json:"detail,omitempty"`
}

/*
Event is the union of every event payload streamed by the judge. Only the
fields relevant to a given "type" are populated; the raw JSON is relayed to
clients untouched.
*/
type Event struct {
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
	WallTime       json.Number `json:"wall_time,omitempty"`
	MemUsage       *int64      `json:"mem_usage,omitempty"`
	Reason         string      `json:"reason,omitempty"`
	StatusMessage  string      `json:"status_message,omitempty"`
	UserOutput     string      `json:"user_output,omitempty"`
	UserOutputType string      `json:"user_output_type,omitempty"`
	ErrorMessage   string      `json:"error_message,omitempty"`

	// timings
	Phases []EventPhase `json:"phases,omitempty"`

	// artifact
	Kind          string `json:"kind,omitempty"`
	URL           string `json:"url,omitempty"`
	ArtifactState string `json:"state,omitempty"`
	LimitBytes    *int64 `json:"limit_bytes,omitempty"`

	// alert
	// Code is the stable identifier of the condition ("startup_timeout");
	// Severity is one of the alerts package's severities; Details is the
	// producer's structured context and is stored untouched.
	Code     string          `json:"code,omitempty"`
	Severity string          `json:"severity,omitempty"`
	Details  json.RawMessage `json:"details,omitempty"`

	// finished
	NumCorrectCases    *int64      `json:"num_correct_cases,omitempty"`
	Score              json.Number `json:"score,omitempty"`
	CompilationMessage string      `json:"compilation_message,omitempty"`
	CompilationError   string      `json:"compilation_error,omitempty"`
	StartedAt          string      `json:"started_at,omitempty"`
	FinishedAt         string      `json:"finished_at,omitempty"`
}

// terminalStatuses are the commit statuses from which a commit never moves
// away. Every persistence path, including the authoritative "finished" event,
// is guarded so a delayed or stale event cannot revive a settled commit.
var terminalStatuses = map[string]bool{
	"completed":         true,
	"uncompleted":       true,
	"compilation_error": true,
	"server_error":      true,
	"timeout":           true,
	"plagiarism":        true,
}

// nonTerminalGuard is the SQL predicate used to protect terminal commits.
const nonTerminalGuard = `status NOT IN ('completed','uncompleted','compilation_error','server_error','timeout','plagiarism')`

/*
IsTerminalStatus reports whether a commit status is final.
*/
func IsTerminalStatus(status string) bool {
	return terminalStatuses[status]
}

/*
PersistEvent writes a judge event to Postgres. Every update is guarded so a
terminal status is never overwritten by a later event.
*/
func PersistEvent(ctx context.Context, ev *Event) error {
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
	case "timings":
		return persistTimings(ctx, ev)
	case "alert":
		return persistAlert(ctx, ev)
	case "finished":
		return persistFinished(ctx, ev)
	case "artifact":
		return persistArtifact(ctx, ev)
	case "error":
		// The judge sends `error` as a diagnostic before the authoritative
		// `finished` event. Persisting `server_error` here would make that
		// terminal event a no-op and turn a genuine timeout into an
		// infrastructure error. If the stream ends without `finished`, the hub's
		// failServerError path settles the commit.
		slog.ErrorContext(ctx, "judge reported an error",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("message", ev.Message),
		)
		return nil
	default:
		slog.WarnContext(ctx, "ignoring unknown judge event type",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("type", ev.Type),
		)
		return nil
	}
}

func persistStatus(ctx context.Context, ev *Event) error {
	if ev.Status == "" {
		return fmt.Errorf("status event without a status")
	}

	if _, err := database.DB.ExecContext(ctx,
		`UPDATE commits SET status = $1::commit_status_t
		 WHERE id = $2 AND `+nonTerminalGuard,
		ev.Status, ev.CommitID,
	); err != nil {
		return fmt.Errorf("persisting status event: %w", err)
	}

	return nil
}

func persistCompilation(ctx context.Context, ev *Event) error {
	compiled := false
	if ev.Compiled != nil {
		compiled = *ev.Compiled
	}

	if _, err := database.DB.ExecContext(ctx,
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

/*
persistArtifact records the state of one test case's generated-file upload. The
row is what a reloading client reads to explain a missing download; the
run-level artifact event (files written outside a case's directory) has no case
and only logs its URL.
*/
func persistArtifact(ctx context.Context, ev *Event) error {
	if ev.TestCaseID == 0 {
		slog.InfoContext(ctx, "judge artifact available",
			slog.Int64("commit_id", ev.CommitID),
			slog.String("kind", ev.Kind),
			slog.String("url", ev.URL),
		)
		return nil
	}

	switch ev.ArtifactState {
	case "pending", "ready", "failed":
	default:
		// Older judges sent only the state-less run-level event. Do not make the
		// stream reconnect forever over a field those versions cannot fill.
		slog.WarnContext(ctx, "ignoring artifact event without a state",
			slog.Int64("commit_id", ev.CommitID),
			slog.Int64("test_case_id", ev.TestCaseID),
		)
		return nil
	}

	var reason any
	if ev.Reason != "" {
		reason = ev.Reason
	}
	var limit any
	if ev.LimitBytes != nil {
		limit = *ev.LimitBytes
	}

	if _, err := database.DB.ExecContext(ctx,
		`INSERT INTO commit_test_case_artifacts
		     (commit_id, exercise_test_case_id, state, reason, limit_bytes,
		      updated_at)
		 VALUES ($1, $2, $3::commit_artifact_state_t, $4, $5, now())
		 ON CONFLICT (commit_id, exercise_test_case_id) DO UPDATE SET
		     state = EXCLUDED.state,
		     reason = EXCLUDED.reason,
		     limit_bytes = EXCLUDED.limit_bytes,
		     updated_at = now()`,
		ev.CommitID, ev.TestCaseID, ev.ArtifactState, reason, limit,
	); err != nil {
		return fmt.Errorf("persisting artifact event: %w", err)
	}

	return nil
}

func persistCaseResult(ctx context.Context, ev *Event) error {
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

	var wallTime any
	if ev.WallTime != "" {
		wallTime = ev.WallTime.String()
	}

	userOutputType := ev.UserOutputType
	if userOutputType == "" {
		userOutputType = "text"
	}

	if _, err := database.DB.ExecContext(ctx,
		`INSERT INTO commits_exercise_test_cases_results
		     (commit_id, exercise_test_case_id, cpu_time, wall_time, mem_usage,
		      user_output, user_output_type, status, reason, status_message,
		      error_message)
		 SELECT $1, $2, $3, $4, $5, $6, $7::input_output_type_t,
		        $8::commit_exercise_test_case_status_t,
		        NULLIF($9, '')::case_interruption_reason_t, $10, $11
		 WHERE EXISTS (
		     SELECT 1 FROM commits WHERE id = $1 AND `+nonTerminalGuard+`
		 )
		 ON CONFLICT (commit_id, exercise_test_case_id) DO UPDATE SET
		     cpu_time = EXCLUDED.cpu_time,
		     wall_time = EXCLUDED.wall_time,
		     mem_usage = EXCLUDED.mem_usage,
		     user_output = EXCLUDED.user_output,
		     user_output_type = EXCLUDED.user_output_type,
		     status = EXCLUDED.status,
		     reason = EXCLUDED.reason,
		     status_message = EXCLUDED.status_message,
		     error_message = EXCLUDED.error_message`,
		ev.CommitID, ev.TestCaseID, cpuTime, wallTime, memUsage, ev.UserOutput,
		userOutputType, ev.Status, ev.Reason, ev.StatusMessage, ev.ErrorMessage,
	); err != nil {
		return fmt.Errorf("persisting case_result event: %w", err)
	}

	return nil
}

/*
persistTimings writes the judge's phase timings. The rows are idempotent: a
replayed event updates the same (commit, phase) keys. The event arrives before
`finished`, so a run's timings are durable even when grading later fails.
*/
func persistTimings(ctx context.Context, ev *Event) error {
	if len(ev.Phases) == 0 {
		return nil
	}

	// Validate before opening a transaction, so a malformed event cannot leave
	// a half-written timing set or need a database round trip to be rejected.
	for _, phase := range ev.Phases {
		if phase.Phase == "" {
			return fmt.Errorf("malformed timings event: empty phase")
		}
		if phase.DurationMS < 0 {
			return fmt.Errorf("malformed timings event: negative duration for %q", phase.Phase)
		}
	}

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin timings transaction: %w", err)
	}
	defer tx.Rollback()

	for _, phase := range ev.Phases {
		source := phase.Source
		if source == "" {
			source = "judge"
		}

		var detail any
		if len(phase.Detail) > 0 {
			detail = string(phase.Detail)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO commit_phase_timings
			     (commit_id, phase, duration_ms, cpu_ms, mem_peak_bytes,
			      started_at, finished_at, source, detail)
			 VALUES ($1, $2::commit_phase_t, $3, $4, $5, $6, $7, $8, $9::jsonb)
			 ON CONFLICT (commit_id, phase) DO UPDATE SET
			     duration_ms = EXCLUDED.duration_ms,
			     cpu_ms = EXCLUDED.cpu_ms,
			     mem_peak_bytes = EXCLUDED.mem_peak_bytes,
			     started_at = EXCLUDED.started_at,
			     finished_at = EXCLUDED.finished_at,
			     source = EXCLUDED.source,
			     detail = EXCLUDED.detail`,
			ev.CommitID, phase.Phase, phase.DurationMS, phase.CPUMS,
			phase.MemPeakBytes, phase.StartedAt, phase.FinishedAt, source,
			detail,
		); err != nil {
			return fmt.Errorf("persisting timing %q: %w", phase.Phase, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit timings transaction: %w", err)
	}

	return nil
}

/*
persistAlert records an operational alert the judge raised for a run it could not
complete.

The insert is idempotent per (commit, code), which is what makes a replayed event
safe: a reconnect resends the frame, the row is not duplicated and the
administrators are not notified twice. Notifying them is asynchronous (the alert
package owns that), so a slow or unavailable SMTP relay cannot stall the judge
event stream.
*/
func persistAlert(ctx context.Context, ev *Event) error {
	in, err := alertInput(ev)
	if err != nil {
		return err
	}

	if _, _, err := alerts.Create(ctx, in); err != nil {
		return fmt.Errorf("persisting alert event: %w", err)
	}

	return nil
}

/*
alertInput maps an `alert` event to the alert to record. It is separate from
persistAlert so the validation and the defaults can be asserted without a
database.
*/
func alertInput(ev *Event) (alerts.Input, error) {
	if ev.Code == "" || ev.Message == "" {
		return alerts.Input{}, fmt.Errorf("malformed alert event")
	}

	severity := ev.Severity
	if severity == "" {
		severity = alerts.SeverityWarning
	}

	// A judge always names the commit; a zero id would be a malformed event and
	// an alert about nothing, so it is recorded without a target instead.
	var commitID *int64
	if ev.CommitID > 0 {
		id := ev.CommitID
		commitID = &id
	}

	return alerts.Input{
		Severity: severity,
		Code:     ev.Code,
		Source:   alerts.SourceJudge,
		CommitID: commitID,
		Message:  ev.Message,
		Details:  ev.Details,
	}, nil
}

func persistFinished(ctx context.Context, ev *Event) error {
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

	if _, err := database.DB.ExecContext(ctx,
		`UPDATE commits
		 SET status = $1::commit_status_t,
		     num_correct_cases = $2,
		     score = $3,
		     compilation_message = COALESCE(NULLIF($4, ''), compilation_message),
		     compilation_error = COALESCE(NULLIF($5, ''), compilation_error),
		     compilation_finished = COALESCE(compilation_finished, now())
		 WHERE id = $6 AND `+nonTerminalGuard,
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
	if _, err := database.DB.ExecContext(ctx,
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
