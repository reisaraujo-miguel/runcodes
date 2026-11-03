package services

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

	one := []limitAlertCase{{Number: 3, Failures: 1}}
	if got, want := formatLimitAlertCases(one), "caso 3 (1 submissão)"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	three := []limitAlertCase{
		{Number: 18, Failures: 12},
		{Number: 19, Failures: 7},
		{Number: 20, Failures: 5},
	}
	want := "caso 18 (12 submissões), caso 19 (7 submissões), caso 20 (5 submissões)"
	if got := formatLimitAlertCases(three); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// The email names the worst cases and counts the rest.
	many := make([]limitAlertCase, 0, 8)
	for i := 1; i <= 8; i++ {
		many = append(many, limitAlertCase{Number: i, Failures: 10 - i})
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
