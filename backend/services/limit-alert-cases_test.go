package services

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/models"
)

// alertTestWindow returns a fixed timestamp offset from the same instant, so the
// window bounds in a test are deterministic.
func alertTestWindow(minutes int) time.Time {
	return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

func TestFormatLimitAlertCases(t *testing.T) {
	if got := formatLimitAlertCases(nil); got != "" {
		t.Fatalf("formatLimitAlertCases(nil) = %q, want empty", got)
	}

	one := []models.LimitAlertCase{{Number: 3, Failures: 1}}
	if got, want := formatLimitAlertCases(one), "caso 3 (1 submissão)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	three := []models.LimitAlertCase{
		{Number: 18, Failures: 12},
		{Number: 19, Failures: 7},
		{Number: 20, Failures: 5},
	}
	want := "caso 18 (12 submissões), caso 19 (7 submissões), caso 20 (5 submissões)"
	if got := formatLimitAlertCases(three); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// The email names the worst cases and counts the rest.
	many := make([]models.LimitAlertCase, 0, 8)
	for i := 1; i <= 8; i++ {
		many = append(many, models.LimitAlertCase{Number: i, Failures: 10 - i})
	}
	got := formatLimitAlertCases(many)
	want = "caso 1 (9 submissões), caso 2 (8 submissões), caso 3 (7 submissões), " +
		"caso 4 (6 submissões), caso 5 (5 submissões) e mais 3 casos"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLimitAlertCases(t *testing.T) {
	mock := newMockDB(t)

	limitID := int64(4)
	group := limitFailureGroup{
		ExerciseID:        9,
		AllowedFileTypeID: &limitID,
		LimitName:         "memory",
		WindowStart:       alertTestWindow(10),
		WindowEnd:         alertTestWindow(20),
	}

	mock.ExpectQuery(`WITH numbered AS`).
		WithArgs(int64(12), int64(9), &limitID, "memory_limit", group.WindowStart, group.WindowEnd).
		WillReturnRows(sqlmock.NewRows([]string{"number", "failures"}).
			AddRow(7, 4).
			AddRow(2, 1))

	cases := limitAlertCases(context.Background(), group, 12)
	if len(cases) != 2 {
		t.Fatalf("cases = %v, want two", cases)
	}
	if cases[0].Number != 7 || cases[0].Failures != 4 {
		t.Fatalf("cases[0] = %+v, want case 7 with 4 failures", cases[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestLimitReasonInverse pins that every stored reason has a display name and
// that the reverse lookup used by the per-case query agrees with it.
func TestLimitReasonInverse(t *testing.T) {
	for reason, limit := range limitReasonNames {
		if got := limitReason(limit); got != reason {
			t.Errorf("limitReason(%q) = %q, want %q", limit, got, reason)
		}
	}
	if got := limitReason("not-a-limit"); got != "" {
		t.Fatalf("limitReason(unknown) = %q, want empty", got)
	}
}

// The in-app alert must name the affected cases the same way the email does, so
// the record it is read back from has to carry them.
func TestRaiseLimitAlertStoresTheAffectedCases(t *testing.T) {
	mock := newMockDB(t)

	limitID := int64(4)
	group := limitFailureGroup{
		ExerciseID:        9,
		AllowedFileTypeID: &limitID,
		LimitName:         "memory",
		Failures:          5,
		WindowStart:       alertTestWindow(10),
		WindowEnd:         alertTestWindow(20),
	}
	cfg := config.LimitsConfig{AlertCooldown: 6 * time.Hour}

	mock.ExpectQuery(`SELECT COALESCE\(e.real_id, e.id\), e.title`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "email", "name"}).
			AddRow(int64(9), "Cosmic Dust", "", "Turma A"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM exercise_limit_alerts`).
		WithArgs(int64(9), &limitID, "memory", "6h0m0s").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT name FROM allowed_file_types`).
		WithArgs(limitID).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Python"))
	mock.ExpectQuery(`WITH numbered AS`).
		WithArgs(int64(9), int64(9), &limitID, "memory_limit",
			group.WindowStart, group.WindowEnd).
		WillReturnRows(sqlmock.NewRows([]string{"number", "failures"}).
			AddRow(7, 4).
			AddRow(2, 1))
	mock.ExpectQuery(`INSERT INTO exercise_limit_alerts`).
		WithArgs(int64(9), &limitID, "memory", 5,
			`[{"number":7,"failures":4},{"number":2,"failures":1}]`,
			group.WindowStart, group.WindowEnd).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(31)))

	raiseLimitAlert(context.Background(), group, cfg)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
