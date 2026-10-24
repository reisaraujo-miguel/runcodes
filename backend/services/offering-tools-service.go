package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/storage"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
)

const (
	// MaxOfferingEmailSubjectLength and MaxOfferingEmailMessageLength bound the
	// message a professor can send to every member of a class.
	MaxOfferingEmailSubjectLength = 200
	MaxOfferingEmailMessageLength = 20000

	// maxBulkRecompile bounds one recompilation request. It is high enough for a
	// large class and low enough that a misclick cannot queue tens of thousands
	// of runs at once.
	maxBulkRecompile = 5000
)

// latestSubmission is one row of the "last submission" report: the newest source
// a student sent for one exercise.
type latestSubmission struct {
	CommitID    int64
	UserID      int64
	StudentName string
	StudentMail string
	ExerciseID  int64
	Title       string
	S3Key       string
	Score       float64
	Status      string
	CreatedAt   time.Time
}

/*
AuthorizeOfferingAuthor reports whether the caller may read the offering's grade
and submissions data. Handlers that stream a download use it before writing the
response headers, so an unauthorized caller still receives a normal JSON error
instead of a broken zip.
*/
func AuthorizeOfferingAuthor(
	ctx context.Context, offeringID int64, claims map[string]any,
) error {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return err
	}
	if !rel.CanAuthor() {
		return ErrOfferingNotFound
	}
	return nil
}

/*
WriteOfferingSubmissionsArchive streams a zip with the newest submission of every
student in the offering, grouped by student and exercise, together with a CSV
report of the grades.

The archive is streamed because submissions can be large; only the small CSV is
built in memory. A source object that is missing from storage is replaced by a
note inside its folder, so one lost object does not abort the download.
*/
func WriteOfferingSubmissionsArchive(
	ctx context.Context, offeringID int64, claims map[string]any, w io.Writer,
) (string, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return "", err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return "", err
	}
	if !rel.CanAuthor() {
		return "", ErrOfferingNotFound
	}

	rows, err := loadLatestSubmissions(ctx, offeringID)
	if err != nil {
		return "", err
	}

	writer := zip.NewWriter(w)

	report, err := latestSubmissionsCSV(rel.Offering.Name, rows)
	if err != nil {
		writer.Close()
		return "", ErrServer
	}
	if err := writeZipEntry(writer, "notas.csv", report); err != nil {
		writer.Close()
		return "", ErrServer
	}
	if err := writeZipEntry(
		writer, "LEIA-ME.txt", []byte(submissionsArchiveReadme(rel.Offering.Name)),
	); err != nil {
		writer.Close()
		return "", ErrServer
	}

	studentFolders := make(map[int64]string)
	for _, submission := range rows {
		folder, ok := studentFolders[submission.UserID]
		if !ok {
			folder = fmt.Sprintf(
				"alunos/%d_%s", submission.UserID,
				archiveSafeName(submission.StudentName),
			)
			studentFolders[submission.UserID] = folder
		}

		entry := fmt.Sprintf(
			"%s/%d_%s/%s", folder, submission.ExerciseID,
			archiveSafeName(submission.Title),
			archiveSafeName(path.Base(submission.S3Key)),
		)

		content, readErr := readCommitObject(ctx, submission.S3Key)
		if readErr != nil {
			note := fmt.Sprintf(
				"O arquivo desta submissão não está mais disponível no armazenamento.\n"+
					"Aluno: %s <%s>\nExercício: %s\nSubmissão: #%d\n",
				submission.StudentName, submission.StudentMail,
				submission.Title, submission.CommitID,
			)
			if err := writeZipEntry(writer, entry+".indisponivel.txt", []byte(note)); err != nil {
				writer.Close()
				return "", ErrServer
			}
			continue
		}

		if err := writeZipEntry(writer, entry, content); err != nil {
			writer.Close()
			return "", ErrServer
		}
	}

	if err := writer.Close(); err != nil {
		slog.ErrorContext(ctx, "error finishing the submissions archive",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}

	return fmt.Sprintf("turma-%d-ultimas-submissoes.zip", offeringID), nil
}

func submissionsArchiveReadme(offeringName string) string {
	return "Últimas submissões — " + offeringName + "\n\n" +
		"Cada pasta em alunos/ corresponde a um aluno, e cada subpasta corresponde\n" +
		"a um exercício. O arquivo é a submissão mais recente do aluno naquele\n" +
		"exercício. A nota em notas.csv é a nota dessa submissão, não a melhor\n" +
		"nota histórica do aluno.\n\n" +
		"Se um arquivo não está mais disponível no armazenamento, a pasta traz um\n" +
		"arquivo .indisponivel.txt com os dados da submissão.\n"
}

func loadLatestSubmissions(
	ctx context.Context, offeringID int64,
) ([]latestSubmission, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (c.user_id, c.exercise_id)
		       c.id, c.user_id, u.name, u.email, e.id, e.title,
		       c.s3_key, c.score::float8, c.status::text, c.created_at
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		JOIN users u ON u.id = c.user_id
		JOIN enrollments en
		  ON en.user_id = c.user_id AND en.offering_id = $1
		 AND en.role = 'student' AND NOT en.banned
		WHERE e.offering_id = $1 AND NOT e.removed
		  AND c.s3_key IS NOT NULL AND c.s3_key <> ''
		ORDER BY c.user_id, c.exercise_id, c.created_at DESC, c.id DESC`,
		offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading the latest submissions",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	submissions := make([]latestSubmission, 0, 64)
	for rows.Next() {
		var submission latestSubmission
		if err := rows.Scan(
			&submission.CommitID, &submission.UserID, &submission.StudentName,
			&submission.StudentMail, &submission.ExerciseID, &submission.Title,
			&submission.S3Key, &submission.Score, &submission.Status,
			&submission.CreatedAt,
		); err != nil {
			slog.ErrorContext(ctx, "error scanning the latest submission",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return submissions, nil
}

func latestSubmissionsCSV(
	offeringName string, submissions []latestSubmission,
) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.UseCRLF = true

	if err := writer.Write([]string{
		"Turma", "Aluno", "Email", "Exercício", "Nota da última submissão",
		"Situação", "Enviada em", "Submissão",
	}); err != nil {
		return nil, err
	}
	for _, submission := range submissions {
		if err := writer.Write([]string{
			csvSafe(offeringName),
			csvSafe(submission.StudentName),
			csvSafe(submission.StudentMail),
			csvSafe(submission.Title),
			strconv.FormatFloat(submission.Score, 'f', 2, 64),
			csvSafe(submission.Status),
			submission.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
			strconv.FormatInt(submission.CommitID, 10),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buffer.Bytes(), writer.Error()
}

// archiveSafeName turns a user-controlled name into a single path segment:
// letters, digits, dots, dashes and underscores survive; everything else becomes
// an underscore. Empty names fall back to "arquivo".
func archiveSafeName(name string) string {
	name = strings.TrimSpace(name)
	var builder strings.Builder
	lastUnderscore := false
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '.', char == '-', char == '_':
			builder.WriteRune(char)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	value := strings.Trim(builder.String(), "._-")
	if value == "" {
		return "arquivo"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	return value
}

func readCommitObject(ctx context.Context, key string) ([]byte, error) {
	object, err := storage.GetCommitObject(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, err
		}
		slog.ErrorContext(ctx, "error reading a submitted source",
			slog.String("s3_key", key),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer object.Body.Close()

	data, err := io.ReadAll(io.LimitReader(object.Body, MaxSubmissionBytes+1))
	if err != nil {
		return nil, ErrServer
	}
	if len(data) > MaxSubmissionBytes {
		return nil, ErrServer
	}
	return data, nil
}

/*
RecompileOfferingSubmissions queues a fresh run of the newest submission of every
student on every exercise. A new commit row is created instead of resetting the
old one: the original result stays in the history and the judge's replayable
event stream, which is already finished for that commit id, does not have to be
reopened.

The wake-up calls run in the background because a class can have hundreds of
submissions; the HTTP response reports how many runs were queued.
*/
func RecompileOfferingSubmissions(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.RecompileResponse, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		return nil, ErrOfferingNotFound
	}

	if err := judge.Ready(ctx); err != nil {
		slog.ErrorContext(ctx, "judge is not ready for a bulk recompilation",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrJudgeUnavailable
	}

	sources, err := loadLatestSubmissionIDs(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return &models.RecompileResponse{}, nil
	}
	if len(sources) > maxBulkRecompile {
		sources = sources[:maxBulkRecompile]
	}

	newIDs, err := enqueueRecompilations(ctx, sources)
	if err != nil {
		return nil, err
	}

	// The response must not wait on the judge: each wake-up has its own timeout
	// and a large class would keep the request open for minutes. The detached
	// context keeps the work alive after the handler returns.
	go wakeRecompiledCommits(newIDs)

	slog.InfoContext(ctx, "bulk recompilation queued",
		slog.Int64("offering_id", offeringID),
		slog.Int("commits", len(newIDs)),
	)

	return &models.RecompileResponse{Queued: len(newIDs)}, nil
}

func loadLatestSubmissionIDs(
	ctx context.Context, offeringID int64,
) ([]int64, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (c.user_id, c.exercise_id) c.id
		FROM commits c
		JOIN exercises e ON e.id = c.exercise_id
		JOIN enrollments en
		  ON en.user_id = c.user_id AND en.offering_id = $1
		 AND en.role = 'student' AND NOT en.banned
		WHERE e.offering_id = $1 AND NOT e.removed
		  AND c.s3_key IS NOT NULL AND c.s3_key <> ''
		ORDER BY c.user_id, c.exercise_id, c.created_at DESC, c.id DESC`,
		offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error selecting submissions to recompile",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	ids := make([]int64, 0, 64)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, ErrServer
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return ids, nil
}

func enqueueRecompilations(
	ctx context.Context, sourceIDs []int64,
) ([]int64, error) {
	rows, err := database.DB.QueryContext(ctx, `
		INSERT INTO commits (user_id, exercise_id, status, s3_key)
		SELECT user_id, exercise_id, 'queued', s3_key
		FROM commits
		WHERE id = ANY($1)
		  AND s3_key IS NOT NULL AND s3_key <> ''
		RETURNING id`, pq.Array(sourceIDs))
	if err != nil {
		slog.ErrorContext(ctx, "error enqueueing recompilations",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	newIDs := make([]int64, 0, len(sourceIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, ErrServer
		}
		newIDs = append(newIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return newIDs, nil
}

func wakeRecompiledCommits(commitIDs []int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	for _, commitID := range commitIDs {
		// Subscribe before waking: the new result must be persisted even when
		// nobody is watching the submission. If the wake-up fails but the judge
		// is alive, its poll loop still claims the queued row and this stream
		// receives the events.
		judge.Consume(commitID)

		wakeCtx, wakeCancel := context.WithTimeout(ctx, wakeTimeout)
		err := judge.Wake(wakeCtx, commitID)
		wakeCancel()

		if err != nil {
			slog.ErrorContext(ctx, "could not wake the judge for a recompilation",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			// The row is still queued, so the judge's poll loop can pick it up
			// eventually; deleting it here would lose a run the professor asked
			// for because of a transient wake-up failure.
		}
	}
}

/*
SendOfferingEmail emails every non-banned member of an offering, plus its owner.
Delivery runs in the background and the response reports how many recipients were
queued: a class of a hundred students would otherwise keep the request open past
the browser's timeout.
*/
func SendOfferingEmail(
	ctx context.Context, offeringID int64, req *models.OfferingEmailRequest,
	claims map[string]any,
) (*models.OfferingEmailResponse, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanAuthor() {
		return nil, ErrOfferingNotFound
	}

	req.Subject = strings.TrimSpace(req.Subject)
	if err := validation.ValidateRequiredString(
		req.Subject, MaxOfferingEmailSubjectLength,
	); err != nil {
		return nil, err
	}
	if strings.ContainsAny(req.Subject, "\r\n") {
		return nil, fmt.Errorf("%w: the subject cannot contain a line break",
			ErrInvalidEmail)
	}
	if err := validation.ValidateRequiredString(
		req.Message, MaxOfferingEmailMessageLength,
	); err != nil {
		return nil, err
	}

	recipients, err := loadOfferingRecipients(ctx, offeringID)
	if err != nil {
		return nil, err
	}

	message := mailer.Message{
		Subject: req.Subject,
		Text:    req.Message + "\n\n— " + rel.Offering.Name,
		HTML:    offeringEmailHTML(rel.Offering.Name, req.Message),
	}
	go sendOfferingEmail(recipients, message)

	slog.InfoContext(ctx, "offering email queued",
		slog.Int64("offering_id", offeringID),
		slog.Int("recipients", len(recipients)),
	)

	return &models.OfferingEmailResponse{Recipients: len(recipients)}, nil
}

type emailRecipient struct {
	Name  string
	Email string
}

func loadOfferingRecipients(
	ctx context.Context, offeringID int64,
) ([]emailRecipient, error) {
	rows, err := database.DB.QueryContext(ctx, `
		SELECT DISTINCT u.id, u.name, u.email
		FROM users u
		JOIN offerings o ON o.id = $1 AND o.owner_id = u.id
		UNION
		SELECT u.id, u.name, u.email
		FROM enrollments en
		JOIN users u ON u.id = en.user_id
		WHERE en.offering_id = $1 AND NOT en.banned
		ORDER BY name, email`, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading offering recipients",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	recipients := make([]emailRecipient, 0, 32)
	for rows.Next() {
		var (
			id    int64
			name  string
			email string
		)
		if err := rows.Scan(&id, &name, &email); err != nil {
			return nil, ErrServer
		}
		recipients = append(recipients, emailRecipient{Name: name, Email: email})
	}
	if err := rows.Err(); err != nil {
		return nil, ErrServer
	}
	return recipients, nil
}

func sendOfferingEmail(recipients []emailRecipient, message mailer.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	failures := 0
	for _, recipient := range recipients {
		personal := message
		personal.To = recipient.Email
		if err := sendMail(ctx, personal); err != nil {
			failures++
			slog.ErrorContext(ctx, "error sending an offering email",
				slog.String("recipient", recipient.Email),
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	slog.InfoContext(ctx, "offering email delivery finished",
		slog.Int("recipients", len(recipients)),
		slog.Int("failures", failures),
	)
}

func offeringEmailHTML(offeringName, message string) string {
	escaped := html.EscapeString(message)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")

	return "<p>" + escaped + "</p><p>— " +
		html.EscapeString(offeringName) + "</p>"
}
