/*
Package alerts records operational alerts for the platform administrators and
notifies them by email.

The alerts themselves are persisted by whoever detects the condition: the judge
event consumer (`alert` SSE event) for a run it could not start, and the backend
for conditions only it can observe. This package owns the storage, the admin
listing and the notification, so both producers behave the same way.

It is a package of its own rather than part of `services` because `services`
already imports the `judge` package for submissions; the judge event consumer
needs to record alerts, so the dependency has to point in the other direction.

Notifications are fire-and-forget: a judge event stream, or an HTTP request, must
never wait on SMTP, and an outage of the mail relay must never make an alert
disappear. The alert row is written first and is the durable record; the email is
the convenience. A cooldown per alert code keeps one broken runner image from
mailing every administrator once per submission.
*/
package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/mailer"
)

// Severities an alert can carry; mirrors the CHECK on system_alerts.severity.
const (
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Sources an alert can come from; mirrors the CHECK on system_alerts.source.
const (
	SourceJudge   = "judge"
	SourceBackend = "backend"
)

const (
	// DefaultPageSize and MaxPageSize bound one page of the admin listing.
	DefaultPageSize = 50
	MaxPageSize     = 200

	// EmailCooldown is the minimum interval between two alert emails about the
	// same code. An image that never starts fails every submission that uses it;
	// without this, one broken runner would mail every administrator once per
	// submission. Every alert is still recorded and listed.
	EmailCooldown = 15 * time.Minute

	// Retention is how long an acknowledged alert is kept. Unacknowledged alerts
	// are never pruned: they are the ones somebody still has to look at.
	Retention = 180 * 24 * time.Hour

	// maxTrackedCodes bounds the in-process cooldown map. Codes are a small
	// closed set in practice; the bound keeps a malfunctioning producer of
	// unique codes from growing the map without limit.
	maxTrackedCodes = 100

	// sendTimeout bounds one notification round (recipient query plus SMTP), so
	// the detached goroutine cannot outlive its usefulness indefinitely.
	sendTimeout = 2 * time.Minute
)

var (
	// ErrNotFound reports an alert id that does not exist.
	ErrNotFound = errors.New("alert not found")

	// ErrServer reports a storage failure. It is deliberately opaque: the
	// database error is logged, never returned to the caller.
	ErrServer = errors.New("internal server error, try again later")
)

// Alert is one operational alert, as the admin panel reads it.
type Alert struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Severity  string    `json:"severity"`
	Code      string    `json:"code"`
	Source    string    `json:"source"`
	// CommitID is the run the alert is about, when it is about one.
	CommitID *int64 `json:"commit_id"`
	Message  string `json:"message"`
	// Details is the structured context the producer sent (language, image,
	// wait seconds, …), kept opaque.
	Details        json.RawMessage `json:"details,omitempty"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at"`
	AcknowledgedBy *int64          `json:"acknowledged_by"`
	// AcknowledgedByName is the administrator who acknowledged the alert.
	AcknowledgedByName string `json:"acknowledged_by_name,omitempty"`
}

// Page is one page of the admin listing, plus the open count the panel shows.
type Page struct {
	OpenCount int64   `json:"open_count"`
	Alerts    []Alert `json:"alerts"`
}

// ListOptions selects one page of the listing.
type ListOptions struct {
	// OnlyOpen hides the acknowledged alerts.
	OnlyOpen bool
	Limit    int
	Offset   int
}

// Input is an alert to record. Code and Message are required; the rest falls
// back to a default.
type Input struct {
	Severity string
	Code     string
	Source   string
	// CommitID ties the alert to a run. Alerts for the same (commit, code) are
	// recorded once, which is what makes a replayed judge event harmless.
	CommitID *int64
	Message  string
	Details  json.RawMessage
}

/*
Create records an alert and, when it was not already recorded, notifies the
administrators.

The returned flag reports whether this call inserted the row: a replay of the
same judge event returns the existing alert and does not notify again.
*/
func Create(ctx context.Context, in Input) (Alert, bool, error) {
	alert, inserted, err := insert(ctx, in)
	if err != nil {
		return Alert{}, false, err
	}
	if !inserted {
		return alert, false, nil
	}

	notify(alert)

	return alert, true, nil
}

/*
insert writes the alert row. It reports false when the (commit, code) pair is
already recorded, in which case it returns the stored row.
*/
func insert(ctx context.Context, in Input) (Alert, bool, error) {
	code := strings.TrimSpace(in.Code)
	message := strings.TrimSpace(in.Message)
	if code == "" || message == "" {
		return Alert{}, false, errors.New("an alert needs a code and a message")
	}

	severity := in.Severity
	if severity == "" {
		severity = SeverityWarning
	}

	source := in.Source
	if source == "" {
		source = SourceBackend
	}

	var details any
	if len(in.Details) > 0 {
		details = string(in.Details)
	}

	var (
		alert     Alert
		commitID  sql.NullInt64
		detailsDB []byte
	)

	err := database.DB.QueryRowContext(ctx, `
		INSERT INTO system_alerts (severity, code, source, commit_id, message, details)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		ON CONFLICT (commit_id, code) WHERE commit_id IS NOT NULL DO NOTHING
		RETURNING id, created_at, severity, code, source, commit_id, message, details`,
		severity, code, source, nullableInt64(in.CommitID), message, details,
	).Scan(
		&alert.ID, &alert.CreatedAt, &alert.Severity, &alert.Code, &alert.Source,
		&commitID, &alert.Message, &detailsDB,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Already recorded (a replayed judge event). The conflicting transaction
		// has committed by the time DO NOTHING returns, so the row is visible.
		return existing(ctx, in.CommitID, code)
	}
	if err != nil {
		slog.ErrorContext(ctx, "error recording an alert",
			slog.String("code", code),
			slog.String("error", err.Error()),
		)
		return Alert{}, false, ErrServer
	}

	alert.CommitID = nullInt64Ptr(commitID)
	alert.Details = rawDetails(detailsDB)

	slog.WarnContext(ctx, "operational alert recorded",
		slog.String("code", alert.Code),
		slog.String("severity", alert.Severity),
		slog.Any("commit_id", alert.CommitID),
		slog.String("message", alert.Message),
	)

	return alert, true, nil
}

// existing reads the alert a duplicate insert collided with.
func existing(ctx context.Context, commitID *int64, code string) (Alert, bool, error) {
	alert, err := scanAlert(database.DB.QueryRowContext(ctx, `
		SELECT a.id, a.created_at, a.severity, a.code, a.source, a.commit_id,
		       a.message, a.details, a.acknowledged_at, a.acknowledged_by,
		       COALESCE(u.name, '')
		FROM system_alerts a
		LEFT JOIN users u ON u.id = a.acknowledged_by
		WHERE a.commit_id = $1 AND a.code = $2`,
		nullableInt64(commitID), code,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Alert{}, false, ErrServer
		}
		slog.ErrorContext(ctx, "error reading a duplicate alert",
			slog.String("code", code),
			slog.String("error", err.Error()),
		)
		return Alert{}, false, ErrServer
	}

	return alert, false, nil
}

/*
List returns one page of alerts, newest first, and the number of alerts still
open. The caller owns the page size policy; Limit is clamped here anyway so a
hand-written query cannot ask for the whole table.
*/
func List(ctx context.Context, options ListOptions) (Page, error) {
	limit := options.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	offset := options.Offset
	if offset < 0 {
		offset = 0
	}

	var page Page

	if err := database.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM system_alerts WHERE acknowledged_at IS NULL`,
	).Scan(&page.OpenCount); err != nil {
		slog.ErrorContext(ctx, "error counting open alerts",
			slog.String("error", err.Error()),
		)
		return Page{}, ErrServer
	}

	rows, err := database.DB.QueryContext(ctx, `
		SELECT a.id, a.created_at, a.severity, a.code, a.source, a.commit_id,
		       a.message, a.details, a.acknowledged_at, a.acknowledged_by,
		       COALESCE(u.name, '')
		FROM system_alerts a
		LEFT JOIN users u ON u.id = a.acknowledged_by
		WHERE (NOT $1::boolean OR a.acknowledged_at IS NULL)
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $2 OFFSET $3`,
		options.OnlyOpen, limit, offset,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error listing alerts",
			slog.String("error", err.Error()),
		)
		return Page{}, ErrServer
	}
	defer rows.Close()

	page.Alerts = make([]Alert, 0, limit)
	for rows.Next() {
		alert, err := scanAlert(rows)
		if err != nil {
			slog.ErrorContext(ctx, "error reading an alert row",
				slog.String("error", err.Error()),
			)
			return Page{}, ErrServer
		}
		page.Alerts = append(page.Alerts, alert)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating alerts",
			slog.String("error", err.Error()),
		)
		return Page{}, ErrServer
	}

	return page, nil
}

/*
Acknowledge marks one alert as seen by the calling administrator. It is
idempotent: acknowledging an already acknowledged alert returns the alert with
its original acknowledgement, so a double click is not an error.
*/
func Acknowledge(ctx context.Context, id int64, adminID int) (Alert, error) {
	if _, err := database.DB.ExecContext(ctx, `
		UPDATE system_alerts
		SET acknowledged_at = NOW(), acknowledged_by = $2
		WHERE id = $1 AND acknowledged_at IS NULL`, id, adminID,
	); err != nil {
		slog.ErrorContext(ctx, "error acknowledging an alert",
			slog.Int64("alert_id", id),
			slog.Int("admin_id", adminID),
			slog.String("error", err.Error()),
		)
		return Alert{}, ErrServer
	}

	alert, err := scanAlert(database.DB.QueryRowContext(ctx, `
		SELECT a.id, a.created_at, a.severity, a.code, a.source, a.commit_id,
		       a.message, a.details, a.acknowledged_at, a.acknowledged_by,
		       COALESCE(u.name, '')
		FROM system_alerts a
		LEFT JOIN users u ON u.id = a.acknowledged_by
		WHERE a.id = $1`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Alert{}, ErrNotFound
	} else if err != nil {
		slog.ErrorContext(ctx, "error reading an acknowledged alert",
			slog.Int64("alert_id", id),
			slog.String("error", err.Error()),
		)
		return Alert{}, ErrServer
	}

	return alert, nil
}

/*
rowScanner is the subset *sql.Row and *sql.Rows share, so the alert columns are
decoded in exactly one place.
*/
type rowScanner interface {
	Scan(dest ...any) error
}

// scanAlert decodes one row of the alert listing.
func scanAlert(row rowScanner) (Alert, error) {
	var (
		alert     Alert
		commitID  sql.NullInt64
		details   []byte
		ackAt     sql.NullTime
		ackBy     sql.NullInt64
		ackByName string
	)

	if err := row.Scan(
		&alert.ID, &alert.CreatedAt, &alert.Severity, &alert.Code, &alert.Source,
		&commitID, &alert.Message, &details, &ackAt, &ackBy, &ackByName,
	); err != nil {
		return Alert{}, err
	}

	alert.CommitID = nullInt64Ptr(commitID)
	alert.Details = rawDetails(details)
	if ackAt.Valid {
		at := ackAt.Time
		alert.AcknowledgedAt = &at
	}
	alert.AcknowledgedBy = nullInt64Ptr(ackBy)
	alert.AcknowledgedByName = ackByName

	return alert, nil
}

// nullableInt64 converts an optional id to the driver's representation.
func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// nullInt64Ptr converts a scanned id back to a pointer, nil when NULL.
func nullInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	id := value.Int64
	return &id
}

// rawDetails keeps the stored JSON opaque, and absent when the column is NULL.
func rawDetails(value []byte) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return json.RawMessage(value)
}

// --- notification -----------------------------------------------------------

// notify is the notification side effect of Create. It is a variable so tests
// can observe it without an SMTP server (and without the database access the
// real notification needs).
var notify = notifyAdmins

/*
notifyAdmins emails every administrator about a newly recorded alert.

It returns immediately: the caller is a judge event stream or an HTTP handler,
neither of which may wait on SMTP. The alert row is already durable, so a failed
email costs a notification, not the record.
*/
func notifyAdmins(alert Alert) {
	now := time.Now()
	if !alertEmails.reserve(alert.Code, now) {
		slog.Info("alert email suppressed by cooldown",
			slog.String("code", alert.Code),
			slog.Duration("cooldown", EmailCooldown),
		)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		sent, failed := sendToAdmins(ctx, alert)
		if sent == 0 {
			// Let the next alert of this code retry: nothing reached anybody.
			alertEmails.release(alert.Code)

			if failed == 0 {
				slog.Warn("no administrator to notify about an alert",
					slog.String("code", alert.Code),
				)
			}
			return
		}

		if failed > 0 {
			slog.Warn("alert email partially delivered",
				slog.String("code", alert.Code),
				slog.Int("sent", sent),
				slog.Int("failed", failed),
			)
		}
	}()
}

/*
sendToAdmins delivers one alert to every confirmed administrator. It returns how
many deliveries succeeded and how many failed; the caller decides whether the
cooldown slot is kept.
*/
func sendToAdmins(ctx context.Context, alert Alert) (sent, failed int) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT email
		FROM users
		WHERE role = 'admin' AND confirmed AND email <> ''
		ORDER BY id`)
	if err != nil {
		slog.ErrorContext(ctx, "error listing administrators to notify",
			slog.String("alert_code", alert.Code),
			slog.String("error", err.Error()),
		)
		return 0, 0
	}
	defer rows.Close()

	var recipients []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			slog.ErrorContext(ctx, "error reading an administrator address",
				slog.String("error", err.Error()),
			)
			return 0, 0
		}
		recipients = append(recipients, email)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating administrators",
			slog.String("error", err.Error()),
		)
		return 0, 0
	}

	subject, text, htmlBody := buildEmail(alert)

	for _, recipient := range recipients {
		err := mailer.Send(ctx, mailer.Message{
			To:      recipient,
			Subject: subject,
			Text:    text,
			HTML:    htmlBody,
		})
		if err != nil {
			failed++
			slog.ErrorContext(ctx, "error sending an alert email",
				slog.String("alert_code", alert.Code),
				slog.String("error", err.Error()),
			)
			continue
		}
		sent++
	}

	return sent, failed
}

/*
buildEmail builds the administrator notification for an alert. It is a pure
function so the wording and the escaping can be tested without a database or an
SMTP server.
*/
func buildEmail(alert Alert) (subject, text, htmlBody string) {
	// A header cannot carry a line break; the code and the severity are the only
	// values that reach the subject, and both are short machine identifiers.
	subject = fmt.Sprintf("[RunCodes] %s: %s",
		strings.Join(strings.Fields(severityLabel(alert.Severity)), " "),
		strings.Join(strings.Fields(alert.Code), " "),
	)

	when := alert.CreatedAt.In(time.Local).Format("02/01/2006 às 15:04")
	target := "—"
	if alert.CommitID != nil {
		target = fmt.Sprintf("submissão %d", *alert.CommitID)
	}

	link := ""
	if base := config.Get().PublicURL; base != "" {
		link = base + "/admin/alerts"
	}

	text = fmt.Sprintf(
		"Um alerta operacional foi registrado na plataforma.\n\n"+
			"Severidade: %s\n"+
			"Código: %s\n"+
			"Origem: %s\n"+
			"Referência: %s\n"+
			"Quando: %s\n\n"+
			"%s\n",
		severityLabel(alert.Severity), alert.Code, alert.Source, target, when,
		alert.Message,
	)
	if len(alert.Details) > 0 {
		text += "\nDetalhes: " + string(alert.Details) + "\n"
	}
	if link != "" {
		text += "\nVeja o painel de alertas em: " + link + "\n"
	}

	htmlBody = fmt.Sprintf(
		"<p>Um alerta operacional foi registrado na plataforma.</p>"+
			"<p><strong>Severidade:</strong> %s<br>"+
			"<strong>Código:</strong> %s<br>"+
			"<strong>Origem:</strong> %s<br>"+
			"<strong>Referência:</strong> %s<br>"+
			"<strong>Quando:</strong> %s</p>"+
			"<p>%s</p>",
		html.EscapeString(severityLabel(alert.Severity)),
		html.EscapeString(alert.Code),
		html.EscapeString(alert.Source),
		html.EscapeString(target),
		html.EscapeString(when),
		html.EscapeString(alert.Message),
	)
	if len(alert.Details) > 0 {
		htmlBody += fmt.Sprintf(
			"<pre>%s</pre>", html.EscapeString(string(alert.Details)),
		)
	}
	if link != "" {
		htmlBody += fmt.Sprintf(
			`<p><a href="%s">Abrir o painel de alertas</a></p>`,
			html.EscapeString(link),
		)
	}

	return subject, text, htmlBody
}

// severityLabel is the human label used in the email body.
func severityLabel(severity string) string {
	if severity == SeverityCritical {
		return "Crítico"
	}
	return "Aviso"
}

/*
StartRetention deletes acknowledged alerts older than Retention, once a day. It
is started by main as a background goroutine; the first pass runs immediately so
a restart does not leave months of history for another day.
*/
func StartRetention(ctx context.Context) {
	go func() {
		pruneAlerts(ctx)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pruneAlerts(ctx)
			}
		}
	}()
}

// pruneAlerts applies the retention window once.
func pruneAlerts(ctx context.Context) {
	cutoff := time.Now().Add(-Retention)

	result, err := database.DB.ExecContext(ctx, `
		DELETE FROM system_alerts
		WHERE acknowledged_at IS NOT NULL AND acknowledged_at < $1`, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "error pruning acknowledged alerts",
			slog.String("error", err.Error()),
		)
		return
	}

	if removed, err := result.RowsAffected(); err == nil && removed > 0 {
		slog.InfoContext(ctx, "pruned acknowledged alerts",
			slog.Int64("removed", removed),
			slog.Time("cutoff", cutoff),
		)
	}
}

// --- email cooldown ---------------------------------------------------------

// alertEmailCooldown is the process-wide cooldown shared by every alert email.
var alertEmails = newCooldown(EmailCooldown, maxTrackedCodes)

/*
cooldown is a small in-process "not more often than once per window" map, keyed
by alert code. A distributed deployment would move it to Redis; the backend is a
single process, and this only has to stop a mail storm.
*/
type cooldown struct {
	mu     sync.Mutex
	last   map[string]time.Time
	window time.Duration
	max    int
}

func newCooldown(window time.Duration, max int) *cooldown {
	return &cooldown{last: make(map[string]time.Time), window: window, max: max}
}

// reserve reports whether key may be used at now, and records it. A full map
// is trimmed of expired entries first; if that does not free a slot, the key is
// refused, which merely suppresses an email.
func (c *cooldown) reserve(key string, now time.Time) bool {
	if key == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if last, ok := c.last[key]; ok && now.Sub(last) < c.window {
		return false
	}

	if len(c.last) >= c.max {
		for existing, used := range c.last {
			if now.Sub(used) >= c.window {
				delete(c.last, existing)
			}
		}
		if len(c.last) >= c.max {
			return false
		}
	}

	c.last[key] = now

	return true
}

// release frees a reservation whose email reached nobody.
func (c *cooldown) release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.last, key)
}
