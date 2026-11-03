package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
A language whose limits are too tight fails submissions in batches, and the
professor only finds out when students complain. This notifier watches the graded
results: when the same exercise and language cross the failure threshold on one
limit inside the window, it records an alert for the exercise page and emails the
exercise author once per cooldown.
*/

// limitAlertInterval is how often the notifier looks for new failures. It is not
// configurable on purpose: the window and the cooldown are what control how
// often a professor hears about the same problem.
const limitAlertInterval = 5 * time.Minute

// limitReasonNames maps a stored case reason to the limit it names.
var limitReasonNames = map[string]string{
	"timeout":         "timeout",
	"cpu_limit":       "cpu_time",
	"memory_limit":    "memory",
	"stack_limit":     "stack",
	"file_size_limit": "file_size",
}

// limitNames is the inverse, for the alert text.
func limitDisplayName(limit string) string {
	switch limit {
	case "cpu_time":
		return "tempo de CPU"
	case "memory":
		return "memória"
	case "stack":
		return "pilha"
	case "file_size":
		return "tamanho de arquivo"
	case "timeout":
		return "tempo limite"
	default:
		return limit
	}
}

// limitReason is the inverse of limitReasonNames, for querying by limit name.
func limitReason(limit string) string {
	for reason, name := range limitReasonNames {
		if name == limit {
			return reason
		}
	}
	return ""
}

// alertCaseLimit caps how many cases the email names, so an exercise where every
// case fails still produces a short message.
const alertCaseLimit = 5

// formatLimitAlertCases renders the affected cases for the email body.
func formatLimitAlertCases(cases []models.LimitAlertCase) string {
	if len(cases) == 0 {
		return ""
	}

	shown := cases
	if len(shown) > alertCaseLimit {
		shown = shown[:alertCaseLimit]
	}

	parts := make([]string, 0, len(shown))
	for _, affected := range shown {
		parts = append(parts, fmt.Sprintf(
			"caso %d (%s)", affected.Number,
			plural(affected.Failures, "submissão", "submissões"),
		))
	}

	text := strings.Join(parts, ", ")
	if remaining := len(cases) - len(shown); remaining > 0 {
		text += " e mais " + plural(remaining, "caso", "casos")
	}

	return text
}

// plural renders n with the matching noun form.
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}

	return fmt.Sprintf("%d %s", n, pluralForm)
}

/* StartLimitAlertNotifier runs the limit-failure aggregation in the background. */
func StartLimitAlertNotifier(ctx context.Context) {
	cfg := config.Get().Limits
	if cfg.AlertThreshold <= 0 {
		slog.Info("limit alert notifier is disabled")
		return
	}

	go func() {
		slog.Info("limit alert notifier started",
			slog.Int("threshold", cfg.AlertThreshold),
			slog.Duration("window", cfg.AlertWindow),
			slog.Duration("cooldown", cfg.AlertCooldown),
		)

		ticker := time.NewTicker(limitAlertInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				raiseLimitAlerts(ctx)
			}
		}
	}()
}

// limitFailureGroup is one (exercise, language, limit) bucket counted in the window.
type limitFailureGroup struct {
	ExerciseID        int64
	AllowedFileTypeID *int64
	LimitName         string
	Failures          int
	WindowStart       time.Time
	WindowEnd         time.Time
}

/* raiseLimitAlerts records an alert for every bucket that crossed the threshold. */
func raiseLimitAlerts(ctx context.Context) {
	cfg := config.Get().Limits

	rows, err := database.DB.QueryContext(ctx, `
		SELECT c.exercise_id, c.allowed_file_type_id, r.reason::text,
		       count(DISTINCT c.id), min(c.created_at), max(c.created_at)
		FROM commits_exercise_test_cases_results r
		JOIN commits c ON c.id = r.commit_id
		WHERE r.reason IN ('timeout', 'cpu_limit', 'memory_limit',
		                   'stack_limit', 'file_size_limit')
		  AND c.created_at >= now() - $1::interval
		GROUP BY c.exercise_id, c.allowed_file_type_id, r.reason
		HAVING count(DISTINCT c.id) >= $2`,
		cfg.AlertWindow.String(), cfg.AlertThreshold,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error aggregating limit failures",
			slog.String("error", err.Error()),
		)
		return
	}
	defer rows.Close()

	groups := make([]limitFailureGroup, 0, 4)
	for rows.Next() {
		var reason string
		var group limitFailureGroup
		if err := rows.Scan(
			&group.ExerciseID, &group.AllowedFileTypeID, &reason,
			&group.Failures, &group.WindowStart, &group.WindowEnd,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning a limit failure group",
				slog.String("error", err.Error()),
			)
			return
		}
		limit, ok := limitReasonNames[reason]
		if !ok {
			continue
		}
		group.LimitName = limit
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return
	}

	for _, group := range groups {
		raiseLimitAlert(ctx, group, cfg)
	}
}

/*
raiseLimitAlert records one alert and emails the author, unless the same
(exercise, language, limit) was already alerted inside the cooldown. Ghost
exercises (a resubmission copy) resolve to the exercise the professor sees.
*/
func raiseLimitAlert(ctx context.Context, group limitFailureGroup, cfg config.LimitsConfig) {
	exerciseID, title, authorEmail, offeringName, err := limitAlertTarget(
		ctx, group.ExerciseID,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.ErrorContext(ctx, "error resolving the limit alert target",
				slog.Int64("exercise_id", group.ExerciseID),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	var alerted int
	if err := database.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM exercise_limit_alerts
		WHERE exercise_id = $1
		  AND allowed_file_type_id IS NOT DISTINCT FROM $2
		  AND limit_name = $3
		  AND created_at >= now() - $4::interval`,
		exerciseID, group.AllowedFileTypeID, group.LimitName,
		cfg.AlertCooldown.String(),
	).Scan(&alerted); err != nil {
		slog.ErrorContext(ctx, "error checking the limit alert cooldown",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return
	}
	if alerted > 0 {
		return
	}

	var languageName string
	if group.AllowedFileTypeID != nil {
		if err := database.DB.QueryRowContext(ctx,
			`SELECT name FROM allowed_file_types WHERE id = $1`,
			*group.AllowedFileTypeID,
		).Scan(&languageName); err != nil && err != sql.ErrNoRows {
			languageName = ""
		}
	}

	// The cases are part of the alert: the email names them, and the in-app alert
	// reads them back from the record instead of re-aggregating a window that may
	// still be filling.
	cases := limitAlertCases(ctx, group, exerciseID)
	if cases == nil {
		cases = []models.LimitAlertCase{}
	}
	casesJSON, err := json.Marshal(cases)
	if err != nil {
		slog.ErrorContext(ctx, "error encoding the limit alert cases",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return
	}

	var alertID int64
	if err := database.DB.QueryRowContext(ctx, `
		INSERT INTO exercise_limit_alerts
		    (exercise_id, allowed_file_type_id, limit_name, failures, cases,
		     window_started_at, window_ended_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING id`,
		exerciseID, group.AllowedFileTypeID, group.LimitName, group.Failures,
		string(casesJSON), group.WindowStart, group.WindowEnd,
	).Scan(&alertID); err != nil {
		slog.ErrorContext(ctx, "error recording a limit alert",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return
	}

	slog.WarnContext(ctx, "limit alert recorded",
		slog.Int64("alert_id", alertID),
		slog.Int64("exercise_id", exerciseID),
		slog.String("limit", group.LimitName),
		slog.Int("failures", group.Failures),
	)

	if authorEmail == "" {
		return
	}
	sendLimitAlertEmail(ctx, group, cases, alertID, exerciseID,
		title, offeringName, languageName, authorEmail)
}

/*
limitAlertTarget resolves the exercise a group belongs to, collapsing a ghost
exercise to its original, and the address and class name for the email.
*/
func limitAlertTarget(
	ctx context.Context, exerciseID int64,
) (id int64, title, email, offeringName string, err error) {
	err = database.DB.QueryRowContext(ctx, `
		SELECT COALESCE(e.real_id, e.id), e.title, u.email, o.name
		FROM exercises e
		JOIN users u ON u.id = e.creator_id
		JOIN offerings o ON o.id = e.offering_id
		WHERE e.id = $1`, exerciseID,
	).Scan(&id, &title, &email, &offeringName)
	return id, title, email, offeringName, err
}

/*
limitAlertCases lists the test cases a group's failures were concentrated in,
worst first, numbered by the exercise the professor sees (a ghost exercise shares
the real exercise's cases, so the numbers match the results panel).
*/
func limitAlertCases(
	ctx context.Context, group limitFailureGroup, exerciseID int64,
) []models.LimitAlertCase {
	reason := limitReason(group.LimitName)
	if reason == "" {
		return nil
	}

	rows, err := database.DB.QueryContext(ctx, `
		WITH numbered AS (
			SELECT id, row_number() OVER (ORDER BY id) AS number
			FROM exercises_test_cases
			WHERE exercise_id = $1
		)
		SELECT n.number, count(DISTINCT r.commit_id) AS failures
		FROM commits_exercise_test_cases_results r
		JOIN commits c ON c.id = r.commit_id
		JOIN numbered n ON n.id = r.exercise_test_case_id
		WHERE c.exercise_id = $2
		  AND c.allowed_file_type_id IS NOT DISTINCT FROM $3
		  AND r.reason = $4::case_interruption_reason_t
		  AND c.created_at >= $5 AND c.created_at <= $6
		GROUP BY n.number
		ORDER BY failures DESC, n.number`, exerciseID, group.ExerciseID,
		group.AllowedFileTypeID, reason, group.WindowStart, group.WindowEnd,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error aggregating the affected test cases",
			slog.Int64("exercise_id", group.ExerciseID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	defer rows.Close()

	cases := make([]models.LimitAlertCase, 0, 4)
	for rows.Next() {
		var affected models.LimitAlertCase
		if err := rows.Scan(&affected.Number, &affected.Failures); err != nil {
			slog.ErrorContext(ctx, "error scanning an affected test case",
				slog.String("error", err.Error()),
			)
			return nil
		}
		cases = append(cases, affected)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating the affected test cases",
			slog.String("error", err.Error()),
		)
		return nil
	}

	return cases
}

// sendLimitAlertEmail tells the exercise author which limit is failing, how many
// submissions it caught and which test cases it is concentrated in, with a link
// to the limits tab.
func sendLimitAlertEmail(
	ctx context.Context, group limitFailureGroup, cases []models.LimitAlertCase,
	alertID, exerciseID int64,
	title, offeringName, languageName, recipient string,
) {
	link := ""
	if base := config.Get().PublicURL; base != "" {
		link = fmt.Sprintf("%s/professor/exercise/%d?tab=limits", base, exerciseID)
	}

	language := languageName
	if language == "" {
		language = "linguagem não identificada"
	}
	limit := limitDisplayName(group.LimitName)
	windowStart := group.WindowStart.In(time.Local).Format("02/01/2006 15:04")
	windowEnd := group.WindowEnd.In(time.Local).Format("02/01/2006 15:04")

	subject := fmt.Sprintf(
		"[RunCodes] Limite de %s pode estar reprovando submissões — %s",
		limit, title,
	)

	// Without the cases, a professor cannot tell whether the limit is too tight
	// for every case or only for one expensive test.
	affected := formatLimitAlertCases(cases)
	casesLine := ""
	casesHTML := ""
	if affected != "" {
		casesLine = fmt.Sprintf("Casos em que o limite foi atingido: %s\n", affected)
		casesHTML = fmt.Sprintf(
			"<p>Casos em que o limite foi atingido: <strong>%s</strong></p>",
			html.EscapeString(affected),
		)
	}

	text := fmt.Sprintf(
		"Olá!\n\n"+
			"%d submissões de %s falharam entre %s e %s por causa do limite de %s.\n\n"+
			"Exercício: %s\n"+
			"Turma: %s\n"+
			"Linguagem: %s\n"+
			"Limite: %s\n"+
			"%s\n"+
			"Se o limite estiver apertado, ajuste-o em Limites no exercício:\n%s\n\n"+
			"Este alerta fica disponível na página do exercício até ser reconhecido.\n",
		group.Failures, language, windowStart, windowEnd, limit,
		title, offeringName, language, limit, casesLine, link,
	)

	htmlBody := fmt.Sprintf(
		"<p>Olá!</p><p><strong>%d submissões</strong> de %s falharam entre %s e %s por causa do limite de <strong>%s</strong>.</p>"+
			"<ul><li>Exercício: %s</li><li>Turma: %s</li><li>Linguagem: %s</li><li>Limite: %s</li></ul>"+
			"%s"+
			"<p>Se o limite estiver apertado, ajuste-o em <em>Limites</em> no exercício:</p><p><a href=%q>Abrir o exercício</a></p>"+
			"<p>Este alerta fica disponível na página do exercício até ser reconhecido.</p>",
		group.Failures, html.EscapeString(language), windowStart, windowEnd,
		html.EscapeString(limit), html.EscapeString(title),
		html.EscapeString(offeringName), html.EscapeString(language),
		html.EscapeString(limit), casesHTML, link,
	)

	if err := mailer.Send(ctx, mailer.Message{
		To: recipient, Subject: subject, Text: text, HTML: htmlBody,
	}); err != nil {
		slog.ErrorContext(ctx, "error sending a limit alert email",
			slog.Int64("alert_id", alertID),
			slog.String("error", err.Error()),
		)
	}
}

/* ListExerciseLimitAlerts returns the unacknowledged alerts of an exercise. */
func ListExerciseLimitAlerts(
	ctx context.Context, exerciseID int64, claims map[string]any,
) ([]models.ExerciseLimitAlert, error) {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return nil, err
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT a.id, a.exercise_id, a.allowed_file_type_id, aft.name,
		       a.limit_name, a.failures, a.cases,
		       a.window_started_at, a.window_ended_at,
		       a.created_at, a.acknowledged_at
		FROM exercise_limit_alerts a
		LEFT JOIN allowed_file_types aft ON aft.id = a.allowed_file_type_id
		WHERE a.exercise_id = $1 AND a.acknowledged_at IS NULL
		ORDER BY a.created_at DESC`, acc.authoringExerciseID())
	if err != nil {
		slog.ErrorContext(ctx, "error listing the limit alerts",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	alerts := make([]models.ExerciseLimitAlert, 0, 4)
	for rows.Next() {
		var (
			alert    models.ExerciseLimitAlert
			casesRaw []byte
		)
		if err := rows.Scan(
			&alert.ID, &alert.ExerciseID, &alert.AllowedFileTypeID,
			&alert.LanguageName, &alert.LimitName, &alert.Failures, &casesRaw,
			&alert.WindowStartedAt, &alert.WindowEndedAt, &alert.CreatedAt,
			&alert.AcknowledgedAt,
		); err != nil {
			return nil, ErrServer
		}
		// The column is NOT NULL with an array default, so a reader can always
		// treat the field as a list; a row that does not decode is logged and
		// reported as "no case detail" instead of leaking a null to the client.
		alert.Cases = []models.LimitAlertCase{}
		if err := json.Unmarshal(casesRaw, &alert.Cases); err != nil {
			slog.ErrorContext(ctx, "error decoding the limit alert cases",
				slog.Int64("alert_id", alert.ID),
				slog.String("error", err.Error()),
			)
			alert.Cases = []models.LimitAlertCase{}
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return alerts, nil
}

/* AcknowledgeExerciseLimitAlert marks one alert as handled. */
func AcknowledgeExerciseLimitAlert(
	ctx context.Context, exerciseID, alertID int64, claims map[string]any,
) error {
	acc, err := requireExerciseAuthor(ctx, exerciseID, claims)
	if err != nil {
		return err
	}

	result, err := database.DB.ExecContext(ctx, `
		UPDATE exercise_limit_alerts
		SET acknowledged_at = now()
		WHERE id = $1 AND exercise_id = $2 AND acknowledged_at IS NULL`,
		alertID, acc.authoringExerciseID())
	if err != nil {
		slog.ErrorContext(ctx, "error acknowledging a limit alert",
			slog.Int64("alert_id", alertID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ErrServer
	}
	if affected == 0 {
		return ErrLimitAlertNotFound
	}
	return nil
}
