package services

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/mailer"
)

const (
	// DeadlineReminderWeekDays and DeadlineReminderFinalDays are the two
	// thresholds the notifier uses, in days before the exercise deadline.
	DeadlineReminderWeekDays  = 7
	DeadlineReminderFinalDays = 3
)

// deadlineReminderExercise is one exercise that entered a reminder window.
type deadlineReminderExercise struct {
	ID           int64
	Title        string
	Deadline     time.Time
	OfferingID   int64
	OfferingName string
}

/*
StartDeadlineNotifier runs the "closing in a week / in three days" email
reminders until ctx is cancelled. It is started by main as a background
goroutine; the first pass runs immediately so a restart does not postpone an
already-due reminder until the first tick.

The configuration can disable it entirely (useful in development, where the
bundled SMTP catcher would otherwise collect mail for the demo data).
*/
func StartDeadlineNotifier(ctx context.Context) {
	cfg := config.Get()
	if !cfg.Notifications.DeadlineReminders {
		slog.Info("deadline email reminders are disabled")
		return
	}

	interval := cfg.Notifications.DeadlineReminderInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}

	go func() {
		slog.Info("deadline email reminders started",
			slog.Duration("interval", interval),
		)

		runDeadlineReminders(ctx, time.Now())

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				runDeadlineReminders(ctx, now)
			}
		}
	}()
}

/*
deadlineReminderBucket reports which reminder an exercise deadline calls for.

A deadline inside three days gets the final reminder; one between three and seven
days away gets the one-week reminder; anything else (including an already-passed
deadline) gets none. The two windows are mutually exclusive, so an exercise that
was never seen during the one-week window does not send both emails in the same
pass after a restart.
*/
func deadlineReminderBucket(deadline, now time.Time) (int, bool) {
	if !deadline.After(now) {
		return 0, false
	}
	if !deadline.After(now.Add(DeadlineReminderFinalDays * 24 * time.Hour)) {
		return DeadlineReminderFinalDays, true
	}
	if !deadline.After(now.Add(DeadlineReminderWeekDays * 24 * time.Hour)) {
		return DeadlineReminderWeekDays, true
	}
	return 0, false
}

func runDeadlineReminders(ctx context.Context, now time.Time) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT e.id, e.title, e.deadline, o.id, o.name
		FROM exercises e
		JOIN offerings o ON o.id = e.offering_id
		WHERE NOT e.removed
		  AND e.open_date <= $1
		  AND e.deadline > $1
		  AND e.deadline <= $2
		ORDER BY e.deadline, e.id`,
		now, now.Add(DeadlineReminderWeekDays*24*time.Hour),
	)
	if err != nil {
		slog.ErrorContext(ctx, "error loading exercises for deadline reminders",
			slog.String("error", err.Error()),
		)
		return
	}

	exercises := make([]deadlineReminderExercise, 0, 16)
	for rows.Next() {
		var exercise deadlineReminderExercise
		if err := rows.Scan(
			&exercise.ID, &exercise.Title, &exercise.Deadline,
			&exercise.OfferingID, &exercise.OfferingName,
		); err != nil {
			rows.Close()
			slog.ErrorContext(ctx, "error scanning an exercise for reminders",
				slog.String("error", err.Error()),
			)
			return
		}
		exercises = append(exercises, exercise)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		slog.ErrorContext(ctx, "error iterating exercises for reminders",
			slog.String("error", err.Error()),
		)
		return
	}
	rows.Close()

	sent, failed := 0, 0
	for _, exercise := range exercises {
		daysBefore, ok := deadlineReminderBucket(exercise.Deadline, now)
		if !ok {
			continue
		}

		students, err := loadStudentsWithoutSubmission(
			ctx, exercise.OfferingID, exercise.ID, daysBefore,
		)
		if err != nil {
			failed++
			continue
		}

		for _, student := range students {
			claimed, err := claimDeadlineReminder(
				ctx, exercise.ID, student.ID, daysBefore,
			)
			if err != nil {
				failed++
				continue
			}
			if !claimed {
				// Another replica (or a previous pass) already sent this one.
				continue
			}

			subject, text, htmlBody := deadlineReminderEmail(
				student.Name, exercise, daysBefore,
			)
			err = sendMail(ctx, mailer.Message{
				To:      student.Email,
				Subject: subject,
				Text:    text,
				HTML:    htmlBody,
			})
			if err != nil {
				failed++
				releaseDeadlineReminder(ctx, exercise.ID, student.ID, daysBefore)
				slog.ErrorContext(ctx, "error sending a deadline reminder",
					slog.Int64("exercise_id", exercise.ID),
					slog.Int64("user_id", student.ID),
					slog.String("error", err.Error()),
				)
				continue
			}
			sent++
		}
	}

	if sent > 0 || failed > 0 {
		slog.InfoContext(ctx, "deadline reminder pass finished",
			slog.Int("sent", sent),
			slog.Int("failed", failed),
		)
	}
}

type reminderStudent struct {
	ID    int64
	Name  string
	Email string
}

func loadStudentsWithoutSubmission(
	ctx context.Context, offeringID, exerciseID int64, daysBefore int,
) ([]reminderStudent, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT u.id, u.name, u.email
		FROM enrollments en
		JOIN users u ON u.id = en.user_id
		WHERE en.offering_id = $1
		  AND en.role = 'student'
		  AND NOT en.banned
		  AND NOT EXISTS (
		      SELECT 1 FROM commits c
		      WHERE c.user_id = en.user_id AND c.exercise_id = $2
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM exercise_deadline_notifications n
		      WHERE n.exercise_id = $2 AND n.user_id = en.user_id
		        AND n.days_before = $3
		  )
		ORDER BY u.name, u.id`,
		offeringID, exerciseID, daysBefore,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error loading students for a deadline reminder",
			slog.Int64("exercise_id", exerciseID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	students := make([]reminderStudent, 0, 32)
	for rows.Next() {
		var student reminderStudent
		if err := rows.Scan(
			&student.ID, &student.Name, &student.Email,
		); err != nil {
			return nil, ErrServer
		}
		students = append(students, student)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return students, nil
}

// claimDeadlineReminder reserves the right to send one reminder. The primary key
// makes the operation a no-op for a duplicate, so two replicas cannot both send
// the same mail.
func claimDeadlineReminder(
	ctx context.Context, exerciseID, userID int64, daysBefore int,
) (bool, error) {
	result, err := database.DB.ExecContext(ctx, `
		INSERT INTO exercise_deadline_notifications
		    (exercise_id, user_id, days_before)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, exerciseID, userID, daysBefore,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error claiming a deadline reminder",
			slog.Int64("exercise_id", exerciseID),
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return false, ErrServer
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, ErrServer
	}
	return affected > 0, nil
}

// releaseDeadlineReminder undoes a claim whose email could not be sent, so the
// next pass retries it instead of silently dropping the notification.
func releaseDeadlineReminder(
	ctx context.Context, exerciseID, userID int64, daysBefore int,
) {
	if _, err := database.DB.ExecContext(ctx, `
		DELETE FROM exercise_deadline_notifications
		WHERE exercise_id = $1 AND user_id = $2 AND days_before = $3`,
		exerciseID, userID, daysBefore,
	); err != nil {
		slog.ErrorContext(ctx, "error releasing a deadline reminder",
			slog.Int64("exercise_id", exerciseID),
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
	}
}

/*
deadlineReminderEmail builds the message. It is a pure function so the wording
(the week/final distinction and the submission link) can be tested without a
database or an SMTP server.
*/
func deadlineReminderEmail(
	studentName string, exercise deadlineReminderExercise, daysBefore int,
) (subject, text, htmlBody string) {
	window := "uma semana"
	if daysBefore == DeadlineReminderFinalDays {
		window = "três dias"
	}

	when := exercise.Deadline.In(time.Local).Format("02/01/2006 às 15:04")
	// A title may contain a line break; the mailer refuses such a subject, and a
	// newline has no place in an email header anyway.
	subjectTitle := strings.Join(strings.Fields(exercise.Title), " ")
	subjectOffering := strings.Join(strings.Fields(exercise.OfferingName), " ")
	subject = fmt.Sprintf(
		"Prazo se aproximando: %s (%s)", subjectTitle, subjectOffering,
	)

	link := ""
	if base := config.Get().PublicURL; base != "" {
		link = fmt.Sprintf("%s/exercises/%d/submit", base, exercise.ID)
	}

	text = fmt.Sprintf(
		"Olá, %s.\n\n"+
			"O prazo do exercício %q, da turma %q, termina em %s, em %s.\n"+
			"Você ainda não fez nenhuma submissão para esse exercício.\n\n",
		studentName, exercise.Title, exercise.OfferingName, window, when,
	)
	if link != "" {
		text += "Envie sua solução em: " + link + "\n\n"
	}
	text += "Se você já enviou uma solução depois deste aviso, ignore esta mensagem.\n"

	htmlBody = fmt.Sprintf(
		"<p>Olá, %s.</p>"+
			"<p>O prazo do exercício <strong>%s</strong>, da turma "+
			"<strong>%s</strong>, termina em <strong>%s</strong>, em %s.</p>"+
			"<p>Você ainda não fez nenhuma submissão para esse exercício.</p>",
		html.EscapeString(studentName), html.EscapeString(exercise.Title),
		html.EscapeString(exercise.OfferingName), window, html.EscapeString(when),
	)
	if link != "" {
		htmlBody += fmt.Sprintf(
			`<p><a href="%s">Enviar solução</a></p>`, html.EscapeString(link),
		)
	}
	htmlBody += "<p>Se você já enviou uma solução depois deste aviso, ignore esta mensagem.</p>"

	return subject, text, htmlBody
}
