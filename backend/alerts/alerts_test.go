package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// alertColumns is the projection every read of an alert uses.
var alertColumns = []string{
	"id", "created_at", "severity", "code", "source", "commit_id", "message",
	"details", "acknowledged_at", "acknowledged_by", "acknowledged_by_name",
}

func newMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

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

	return mock
}

// withPublicURL publishes a configuration so Email can build its panel link.
func withPublicURL(t *testing.T, publicURL string) {
	t.Helper()

	previous := config.C
	config.C = &config.Config{PublicURL: publicURL}
	t.Cleanup(func() { config.C = previous })
}

// stubNotify records the notifications Create would have sent.
func stubNotify(t *testing.T) *[]Alert {
	t.Helper()

	var got []Alert
	previous := notify
	notify = func(alert Alert) { got = append(got, alert) }
	t.Cleanup(func() { notify = previous })

	return &got
}

func int64Ptr(value int64) *int64 { return &value }

var createdAt = time.Date(2026, 11, 2, 12, 37, 0, 0, time.UTC)

// sampleRow is one alert as the listing and the acknowledgement read it.
func sampleRow() *sqlmock.Rows {
	return sqlmock.NewRows(alertColumns).AddRow(
		int64(11), createdAt, SeverityCritical, "startup_timeout", SourceJudge,
		int64(6), "the container did not start",
		[]byte(`{"language":"Python"}`), nil, nil, "",
	)
}

// insertRow is the narrower projection an INSERT ... RETURNING scans.
func insertRow(
	severity, code, source string, commitID any, message string, details any,
) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "created_at", "severity", "code", "source", "commit_id",
		"message", "details",
	}).AddRow(
		int64(11), createdAt, severity, code, source, commitID, message, details,
	)
}

func TestCreateInsertsAndNotifies(t *testing.T) {
	mock := newMockDB(t)
	notified := stubNotify(t)

	details := json.RawMessage(`{"language":"Python"}`)
	mock.ExpectQuery(`INSERT INTO system_alerts`).
		WithArgs(SeverityCritical, "startup_timeout", SourceJudge, int64(6),
			"the container did not start", `{"language":"Python"}`).
		WillReturnRows(insertRow(
			SeverityCritical, "startup_timeout", SourceJudge, int64(6),
			"the container did not start", []byte(`{"language":"Python"}`),
		))

	alert, inserted, err := Create(context.Background(), Input{
		Severity: SeverityCritical,
		Code:     "startup_timeout",
		Source:   SourceJudge,
		CommitID: int64Ptr(6),
		Message:  "the container did not start",
		Details:  details,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !inserted {
		t.Fatal("Create reported a replay for a new alert")
	}
	if alert.ID != 11 || alert.Severity != SeverityCritical || alert.CommitID == nil || *alert.CommitID != 6 {
		t.Fatalf("unexpected alert: %+v", alert)
	}
	if string(alert.Details) != `{"language":"Python"}` {
		t.Fatalf("details = %s", alert.Details)
	}
	if len(*notified) != 1 || (*notified)[0].ID != 11 {
		t.Fatalf("notifications = %+v, want one for alert 11", *notified)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A replayed judge event (the stream reconnects from the last persisted
// sequence) must not record a second row nor notify a second time.
func TestCreateReplayDoesNotNotify(t *testing.T) {
	mock := newMockDB(t)
	notified := stubNotify(t)

	mock.ExpectQuery(`INSERT INTO system_alerts`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(int64(6), "startup_timeout").
		WillReturnRows(sampleRow())

	_, inserted, err := Create(context.Background(), Input{
		Code:     "startup_timeout",
		CommitID: int64Ptr(6),
		Message:  "the container did not start",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inserted {
		t.Fatal("Create reported an insert for a replayed event")
	}
	if len(*notified) != 0 {
		t.Fatalf("notifications = %+v, want none", *notified)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateRequiresCodeAndMessage(t *testing.T) {
	for _, in := range []Input{
		{Message: "no code"},
		{Code: "no_message"},
		{Code: "   ", Message: "blank code"},
	} {
		if _, _, err := Create(context.Background(), in); err == nil {
			t.Fatalf("Create(%+v) accepted an incomplete alert", in)
		}
	}
}

func TestCreateDefaultsSeverityAndSource(t *testing.T) {
	mock := newMockDB(t)
	notified := stubNotify(t)

	mock.ExpectQuery(`INSERT INTO system_alerts`).
		WithArgs(SeverityWarning, "disk_full", SourceBackend, nil,
			"the disk is full", nil).
		WillReturnRows(insertRow(
			SeverityWarning, "disk_full", SourceBackend, nil,
			"the disk is full", nil,
		))

	alert, _, err := Create(context.Background(), Input{
		Code:    "disk_full",
		Message: "the disk is full",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if alert.Severity != SeverityWarning || alert.Source != SourceBackend || alert.CommitID != nil {
		t.Fatalf("defaults not applied: %+v", alert)
	}
	if len(*notified) != 1 || (*notified)[0].Severity != SeverityWarning {
		t.Fatalf("notifications = %+v", *notified)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListClampsPaginationAndCountsOpen(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM system_alerts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(false, MaxPageSize, 0).
		WillReturnRows(sampleRow())

	page, err := List(context.Background(), ListOptions{
		OnlyOpen: false,
		Limit:    10_000,
		Offset:   -4,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.OpenCount != 3 {
		t.Fatalf("OpenCount = %d, want 3", page.OpenCount)
	}
	if len(page.Alerts) != 1 || page.Alerts[0].Code != "startup_timeout" {
		t.Fatalf("alerts = %+v", page.Alerts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListOnlyOpenFilters(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM system_alerts`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(true, DefaultPageSize, 0).
		WillReturnRows(sqlmock.NewRows(alertColumns))

	if _, err := List(context.Background(), ListOptions{OnlyOpen: true}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAcknowledgeMissingAlert(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectExec(`UPDATE system_alerts`).
		WithArgs(int64(99), 7).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(int64(99)).
		WillReturnError(sql.ErrNoRows)

	if _, err := Acknowledge(context.Background(), 99, 7); err != ErrNotFound {
		t.Fatalf("Acknowledge error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAcknowledgeReturnsTheAlert(t *testing.T) {
	mock := newMockDB(t)
	ackedAt := time.Date(2026, 11, 2, 13, 0, 0, 0, time.UTC)

	mock.ExpectExec(`UPDATE system_alerts`).
		WithArgs(int64(11), 7).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`FROM system_alerts a`).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows(alertColumns).AddRow(
			int64(11), time.Date(2026, 11, 2, 12, 37, 0, 0, time.UTC),
			SeverityCritical, "startup_timeout", SourceJudge, int64(6),
			"the container did not start", nil, ackedAt, int64(7), "Ada",
		))

	alert, err := Acknowledge(context.Background(), 11, 7)
	if err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if alert.AcknowledgedAt == nil || alert.AcknowledgedBy == nil ||
		*alert.AcknowledgedBy != 7 || alert.AcknowledgedByName != "Ada" {
		t.Fatalf("unexpected acknowledgement: %+v", alert)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPruneAlerts(t *testing.T) {
	mock := newMockDB(t)

	mock.ExpectExec(`DELETE FROM system_alerts`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 4))

	pruneAlerts(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEmailCarriesTheAlertAndThePanelLink(t *testing.T) {
	withPublicURL(t, "https://codes.example.com")

	alert := Alert{
		Code:      "startup_timeout",
		Severity:  SeverityCritical,
		Source:    SourceJudge,
		CommitID:  int64Ptr(6),
		Message:   "the Python container did not print \"run.start\" within 30s",
		CreatedAt: time.Date(2026, 11, 2, 12, 37, 0, 0, time.UTC),
		Details:   json.RawMessage(`{"language":"Python","wait_seconds":30}`),
	}

	subject, text, htmlBody := buildEmail(alert)

	if !strings.Contains(subject, "startup_timeout") ||
		!strings.Contains(subject, "Crítico") {
		t.Fatalf("subject = %q", subject)
	}
	if strings.ContainsAny(subject, "\r\n") {
		t.Fatalf("subject contains a line break: %q", subject)
	}
	for _, want := range []string{
		"startup_timeout", "submissão 6", "the Python container did not print",
		`{"language":"Python","wait_seconds":30}`,
		"https://codes.example.com/admin/alerts",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text body is missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(htmlBody, "https://codes.example.com/admin/alerts") {
		t.Fatalf("html body is missing the link:\n%s", htmlBody)
	}
}

func TestEmailEscapesHTML(t *testing.T) {
	withPublicURL(t, "")

	alert := Alert{
		Code:      "startup_timeout",
		Severity:  SeverityWarning,
		Message:   `<script>alert("x")</script>`,
		CreatedAt: time.Now(),
	}

	_, text, htmlBody := buildEmail(alert)

	if !strings.Contains(text, `<script>`) {
		t.Fatalf("the text body should keep the message verbatim:\n%s", text)
	}
	if strings.Contains(htmlBody, "<script>") {
		t.Fatalf("the html body did not escape the message:\n%s", htmlBody)
	}
	if !strings.Contains(htmlBody, "&lt;script&gt;") {
		t.Fatalf("the html body is missing the escaped message:\n%s", htmlBody)
	}
}

// The subject must survive an alert code that somehow carries a line break: the
// mailer refuses such a subject, and a header cannot hold one.
func TestEmailSanitizesTheSubject(t *testing.T) {
	withPublicURL(t, "")

	subject, _, _ := buildEmail(Alert{
		Code:      "startup\n_timeout",
		CreatedAt: time.Now(),
	})

	if strings.ContainsAny(subject, "\r\n") {
		t.Fatalf("subject = %q", subject)
	}
}

func TestCooldown(t *testing.T) {
	now := time.Now()
	c := newCooldown(time.Minute, 2)

	if !c.reserve("a", now) {
		t.Fatal("the first reservation was refused")
	}
	if c.reserve("a", now.Add(30*time.Second)) {
		t.Fatal("a reservation inside the window was accepted")
	}
	if !c.reserve("b", now) {
		t.Fatal("an independent key was refused")
	}
	if c.reserve("c", now) {
		t.Fatal("a full table accepted a new key")
	}

	// After the window elapses the key may be reserved again.
	if !c.reserve("a", now.Add(2*time.Minute)) {
		t.Fatal("the key was still cooling down after the window")
	}

	// release frees a reservation whose email reached nobody.
	c.release("a")
	if !c.reserve("a", now.Add(2*time.Minute)) {
		t.Fatal("release did not free the key")
	}
}
