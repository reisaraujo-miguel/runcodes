package services

import (
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

func TestDeadlineReminderBucket(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		deadline   time.Time
		wantDays   int
		wantRemind bool
	}{
		{"past", now.Add(-time.Hour), 0, false},
		{"exactly now", now, 0, false},
		{"one hour", now.Add(time.Hour), 3, true},
		{"two days", now.Add(48 * time.Hour), 3, true},
		{"exactly three days", now.Add(72 * time.Hour), 3, true},
		{"three days and a second", now.Add(72*time.Hour + time.Second), 7, true},
		{"five days", now.Add(5 * 24 * time.Hour), 7, true},
		{"exactly seven days", now.Add(7 * 24 * time.Hour), 7, true},
		{"seven days and a second", now.Add(7*24*time.Hour + time.Second), 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			days, remind := deadlineReminderBucket(test.deadline, now)
			if days != test.wantDays || remind != test.wantRemind {
				t.Fatalf("want (%d, %v), got (%d, %v)",
					test.wantDays, test.wantRemind, days, remind)
			}
		})
	}
}

func TestDeadlineReminderEmail(t *testing.T) {
	previous := config.C
	config.C = &config.Config{PublicURL: "https://runcodes.example"}
	defer func() { config.C = previous }()

	exercise := deadlineReminderExercise{
		ID:           42,
		Title:        "Soma <difícil>",
		OfferingName: "Algoritmos",
		Deadline:     time.Date(2026, 10, 27, 18, 0, 0, 0, time.Local),
	}

	subject, text, htmlBody := deadlineReminderEmail(
		"Ada", exercise, DeadlineReminderFinalDays,
	)
	if !strings.Contains(subject, "Soma <difícil>") ||
		!strings.Contains(subject, "Algoritmos") {
		t.Fatalf("subject = %q", subject)
	}
	if !strings.Contains(text, "três dias") {
		t.Fatalf("text = %q, want the final reminder", text)
	}
	if !strings.Contains(text, "https://runcodes.example/exercises/42/submit") {
		t.Fatalf("text = %q, want the submission link", text)
	}
	if !strings.Contains(htmlBody, "&lt;difícil&gt;") {
		t.Fatalf("html = %q, want escaped title", htmlBody)
	}

	_, weekText, _ := deadlineReminderEmail(
		"Ada", exercise, DeadlineReminderWeekDays,
	)
	if !strings.Contains(weekText, "uma semana") {
		t.Fatalf("text = %q, want the weekly reminder", weekText)
	}
}

func TestDeadlineReminderEmailWithoutPublicURL(t *testing.T) {
	previous := config.C
	config.C = &config.Config{}
	defer func() { config.C = previous }()

	_, text, htmlBody := deadlineReminderEmail("Ada", deadlineReminderExercise{
		ID:           1,
		Title:        "Exercício",
		OfferingName: "Turma",
		Deadline:     time.Now().Add(time.Hour),
	}, DeadlineReminderFinalDays)

	if strings.Contains(text, "http") || strings.Contains(htmlBody, "http") {
		t.Fatalf("expected no link without RUNCODES_PUBLIC_URL: %q / %q", text, htmlBody)
	}
}
