package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// withConfig publishes a configuration with a public URL, so the emails can
// build their class link.
func withConfig(t *testing.T, publicURL string) {
	t.Helper()

	previous := config.C
	config.C = &config.Config{PublicURL: publicURL}
	t.Cleanup(func() { config.C = previous })
}

// captureMail replaces the mail sender for the duration of one test and returns
// the messages it was asked to deliver.
func captureMail(t *testing.T) *[]mailer.Message {
	t.Helper()

	var sent []mailer.Message
	previous := sendMail
	sendMail = func(_ context.Context, msg mailer.Message) error {
		sent = append(sent, msg)
		return nil
	}
	t.Cleanup(func() { sendMail = previous })

	return &sent
}

// stubNotifications replaces the post/comment notifications, so a create test
// does not depend on the recipient queries.
func stubNotifications(t *testing.T) {
	t.Helper()

	previousPost, previousComment := publishPostNotification, publishCommentNotification
	publishPostNotification = func(int64, string, models.BulletinPost) {}
	publishCommentNotification = func(
		int64, string, string, models.BulletinComment, bulletinParticipant,
	) {
	}
	t.Cleanup(func() {
		publishPostNotification = previousPost
		publishCommentNotification = previousComment
	})
}

func TestBulletinNotifyGateBoundsAndThrottles(t *testing.T) {
	gate := &bulletinNotifyGate{
		slots: make(chan struct{}, 1),
		last:  make(map[string]time.Time),
	}
	now := time.Now()

	if !gate.begin("comment:4:8", now) {
		t.Fatal("the first comment notification must be accepted")
	}
	if gate.begin("comment:4:8", now.Add(time.Second)) {
		t.Fatal("a second notification inside the cooldown must be refused")
	}
	gate.done()

	if !gate.begin("comment:4:8", now.Add(bulletinCommentCooldown)) {
		t.Fatal("the cooldown must expire")
	}
	gate.done()

	// The single slot is occupied: another author must be skipped instead of
	// spawning an unbounded goroutine.
	if !gate.begin("", now) {
		t.Fatal("an idle gate must accept a delivery")
	}
	if gate.begin("", now) {
		t.Fatal("a full gate must refuse a second concurrent delivery")
	}
	gate.done()
}

// expectRelation queues the two queries loadOfferingRelation issues: the class
// (the cache is off in tests) and the caller's enrollment.
func expectRelation(
	mock sqlmock.Sqlmock, offeringID, userID int64, ownerID int64, role string,
) {
	mock.ExpectQuery(`SELECT id, name, end_date`).
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "end_date", "description", "enrollment_code",
			"owner_id", "visible_to_enroll",
		}).AddRow(
			offeringID, "Algoritmos", time.Now().Add(24*time.Hour), "", "", ownerID, true,
		))
	// The owner short-circuits before the enrollment lookup.
	if ownerID == userID {
		return
	}

	mock.ExpectQuery(`SELECT COALESCE\(role::text`).
		WithArgs(offeringID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"role", "banned"}).
			AddRow(role, false))
}

func TestCreateOfferingPostStoresThePost(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)
	expectRelation(mock, 4, 7, 7, "")

	createdAt := time.Now()
	mock.ExpectQuery(`INSERT INTO offering_posts`).
		WithArgs(int64(4), int64(7), "Prova na sexta", "A prova cobre grafos.").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(11), createdAt))

	post, err := CreateOfferingPost(context.Background(), 4, map[string]any{
		"id": float64(7), "role": "professor", "name": "Ada",
	}, &models.CreateBulletinPostRequest{
		Title: "  Prova na sexta ",
		Body:  "A prova cobre grafos.\n",
	})
	if err != nil {
		t.Fatalf("CreateOfferingPost: %v", err)
	}
	if post.ID != 11 || post.Title != "Prova na sexta" ||
		post.Body != "A prova cobre grafos." {
		t.Fatalf("unexpected post: %+v", post)
	}
	if post.AuthorName != "Ada" || post.AuthorRole != EnrollmentRoleProfessor {
		t.Fatalf("author = %q/%q", post.AuthorName, post.AuthorRole)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Students read the board and comment on it; only the staff starts a post.
func TestCreateOfferingPostRejectsStudents(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)
	expectRelation(mock, 4, 8, 7, EnrollmentRoleStudent)

	_, err := CreateOfferingPost(context.Background(), 4, map[string]any{
		"id": float64(8), "role": "student", "name": "Grace",
	}, &models.CreateBulletinPostRequest{Title: "Oi", Body: "Posso postar?"})
	if err != ErrBulletinPostForbidden {
		t.Fatalf("err = %v, want ErrBulletinPostForbidden", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A monitor is staff for the board even though it cannot author exercises.
func TestCreateOfferingPostAllowsAMonitor(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)
	expectRelation(mock, 4, 9, 7, EnrollmentRoleMonitor)
	mock.ExpectQuery(`INSERT INTO offering_posts`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(12), time.Now()))

	post, err := CreateOfferingPost(context.Background(), 4, map[string]any{
		"id": float64(9), "role": "professor", "name": "Linus",
	}, &models.CreateBulletinPostRequest{Title: "Monitoria", Body: "Hoje às 18h."})
	if err != nil {
		t.Fatalf("CreateOfferingPost: %v", err)
	}
	if post.AuthorRole != EnrollmentRoleMonitor {
		t.Fatalf("author role = %q, want monitor", post.AuthorRole)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOfferingPostValidatesTheBody(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)

	for _, req := range []*models.CreateBulletinPostRequest{
		{Title: "   ", Body: "corpo"},
		{Title: "título", Body: "   "},
		{Title: strings.Repeat("a", MaxBulletinPostTitleLength+1), Body: "corpo"},
	} {
		// Every attempt loads the class and the caller's relation before it
		// validates the payload.
		expectRelation(mock, 4, 7, 7, "")

		if _, err := CreateOfferingPost(context.Background(), 4, map[string]any{
			"id": float64(7), "role": "professor",
		}, req); err == nil {
			t.Fatalf("CreateOfferingPost(%+v) accepted an invalid post", req)
		}
	}
}

func TestCreateBulletinCommentStoresTheComment(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)
	expectRelation(mock, 4, 8, 7, EnrollmentRoleStudent)

	mock.ExpectQuery(`SELECT title FROM offering_posts`).
		WithArgs(int64(11), int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Prova na sexta"))
	mock.ExpectQuery(`INSERT INTO offering_post_comments`).
		WithArgs(int64(11), nil, int64(8), "Vai ter monitoria?").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(21), time.Now()))

	comment, err := CreateBulletinComment(context.Background(), 4, 11, map[string]any{
		"id": float64(8), "role": "student", "name": "Grace",
	}, &models.CreateBulletinCommentRequest{Body: "Vai ter monitoria?"})
	if err != nil {
		t.Fatalf("CreateBulletinComment: %v", err)
	}
	if comment.ID != 21 || comment.AuthorName != "Grace" ||
		comment.AuthorRole != EnrollmentRoleStudent {
		t.Fatalf("unexpected comment: %+v", comment)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A reply must answer a comment of the same post.
func TestCreateBulletinCommentRejectsAForeignParent(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	stubNotifications(t)
	expectRelation(mock, 4, 8, 7, EnrollmentRoleStudent)

	mock.ExpectQuery(`SELECT title FROM offering_posts`).
		WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Prova na sexta"))
	mock.ExpectQuery(`FROM offering_post_comments c`).
		WithArgs(int64(99), int64(11)).
		WillReturnError(sql.ErrNoRows)

	parent := int64(99)
	_, err := CreateBulletinComment(context.Background(), 4, 11, map[string]any{
		"id": float64(8), "role": "student",
	}, &models.CreateBulletinCommentRequest{Body: "?", ParentCommentID: &parent})
	if err != ErrBulletinCommentNotFound {
		t.Fatalf("err = %v, want ErrBulletinCommentNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A reply notifies the student it answers, and nobody else.
func TestNotifyBulletinCommentRepliesToTheStudent(t *testing.T) {
	withConfig(t, "https://codes.example")
	sent := captureMail(t)

	notifyBulletinComment(4, "Algoritmos", "Prova na sexta", models.BulletinComment{
		AuthorID: 7, AuthorName: "Ada", AuthorRole: EnrollmentRoleProfessor,
		Body: "Sim, sexta às 18h.",
	}, bulletinParticipant{
		ID: 8, Name: "Grace", Email: "grace@example.com", Role: EnrollmentRoleStudent,
	})

	if len(*sent) != 1 {
		t.Fatalf("sent %d messages, want 1: %+v", len(*sent), *sent)
	}
	message := (*sent)[0]
	if message.To != "grace@example.com" {
		t.Fatalf("recipient = %q", message.To)
	}
	if !strings.Contains(message.Subject, "Prova na sexta") {
		t.Fatalf("subject = %q", message.Subject)
	}
	for _, want := range []string{
		"Sim, sexta às 18h.", "Ada", "https://codes.example/classes/4",
	} {
		if !strings.Contains(message.Text, want) {
			t.Fatalf("text is missing %q:\n%s", want, message.Text)
		}
	}
}

// A comment that is not answering a student reaches the class staff, and never
// the author of the comment.
func TestNotifyBulletinCommentNotifiesStaff(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	sent := captureMail(t)

	mock.ExpectQuery(`SELECT DISTINCT u.id, u.name, u.email`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(int64(7), "Ada", "ada@example.com").
			AddRow(int64(9), "Linus", "linus@example.com"))

	notifyBulletinComment(4, "Algoritmos", "Prova na sexta", models.BulletinComment{
		AuthorID: 9, AuthorName: "Linus", AuthorRole: EnrollmentRoleMonitor,
		Body: "A monitoria está confirmada.",
	}, bulletinParticipant{})

	if len(*sent) != 1 {
		t.Fatalf("sent %d messages, want 1 for the staff (the author is excluded)", len(*sent))
	}
	if (*sent)[0].To != "ada@example.com" {
		t.Fatalf("recipient = %q, want the professor", (*sent)[0].To)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// A new post emails every enrolled student, with the post's own text.
func TestNotifyBulletinPostEmailsStudents(t *testing.T) {
	withConfig(t, "https://codes.example")
	mock := newMockDB(t)
	sent := captureMail(t)

	mock.ExpectQuery(`en.role = 'student' AND NOT en.banned`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(int64(8), "Grace", "grace@example.com").
			AddRow(int64(10), "Alan", "alan@example.com"))

	notifyBulletinPost(4, "Algoritmos", models.BulletinPost{
		AuthorID: 7, AuthorName: "Ada", Title: "Prova na sexta",
		Body: "A prova cobre grafos e ordenação.",
	})

	if len(*sent) != 2 {
		t.Fatalf("sent %d messages, want 2", len(*sent))
	}
	message := (*sent)[0]
	if !strings.Contains(message.Subject, "Mural: Prova na sexta") {
		t.Fatalf("subject = %q", message.Subject)
	}
	for _, want := range []string{
		"A prova cobre grafos e ordenação.", "Ada",
		"https://codes.example/classes/4",
	} {
		if !strings.Contains(message.Text, want) {
			t.Fatalf("text is missing %q:\n%s", want, message.Text)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBulletinEmailsEscapeHTMLAndHeaders(t *testing.T) {
	withConfig(t, "")

	subject, _, htmlBody := bulletinPostEmail(4, "Algoritmos", models.BulletinPost{
		AuthorName: "Ada\nLovelace",
		Title:      "Prova\nna sexta",
		Body:       `<script>alert("x")</script>`,
	})
	if strings.ContainsAny(subject, "\r\n") {
		t.Fatalf("subject = %q", subject)
	}
	if strings.Contains(htmlBody, "<script>") {
		t.Fatalf("the html body did not escape the post:\n%s", htmlBody)
	}
	if !strings.Contains(htmlBody, "&lt;script&gt;") {
		t.Fatalf("the html body is missing the escaped post:\n%s", htmlBody)
	}

	_, commentText, _ := bulletinCommentEmail(4, "Algoritmos", "Prova na sexta",
		models.BulletinComment{AuthorName: "Grace", Body: "linha 1\nlinha 2"},
	)
	if !strings.Contains(commentText, "linha 1\nlinha 2") {
		t.Fatalf("the text body should keep line breaks:\n%s", commentText)
	}
}

// A reply to a banned student does not go to them: they no longer have access to
// the class, and the staff is the audience that can still act on the question.
func TestNotifyBulletinCommentSkipsABannedStudent(t *testing.T) {
	withConfig(t, "")
	mock := newMockDB(t)
	sent := captureMail(t)

	mock.ExpectQuery(`SELECT DISTINCT u.id, u.name, u.email`).
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).
			AddRow(int64(7), "Ada", "ada@example.com").
			AddRow(int64(9), "Linus", "linus@example.com"))

	// Ada (the professor) wrote the reply, so only Linus is left to notify.
	notifyBulletinComment(4, "Algoritmos", "Prova na sexta", models.BulletinComment{
		AuthorID: 7, AuthorName: "Ada", Body: "A resposta.",
	}, bulletinParticipant{
		ID: 8, Name: "Grace", Email: "grace@example.com",
		Role: EnrollmentRoleStudent, Banned: true,
	})

	if len(*sent) != 1 || (*sent)[0].To != "linus@example.com" {
		t.Fatalf("sent = %+v, want only the other staff member", *sent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
