package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
)

const (
	// MaxBulletinPostTitleLength, MaxBulletinPostBodyLength and
	// MaxBulletinCommentLength bound what a post or a comment may contain. They
	// are generous for an announcement and a question, and small enough that one
	// member cannot fill the board (or the recipients' inboxes) with a pasted
	// essay.
	MaxBulletinPostTitleLength = 160
	MaxBulletinPostBodyLength  = 8000
	MaxBulletinCommentLength   = 4000

	// bulletinBoardPostLimit bounds one board response. The class page shows the
	// newest posts with their discussions; older ones stay readable through the
	// post count the response carries.
	bulletinBoardPostLimit = 20

	// bulletinEmailTimeout bounds one fan-out. A post to a large class sends one
	// message per student, so this has to survive a slow relay.
	bulletinEmailTimeout = 30 * time.Minute
)

/*
ListOfferingBulletin returns a class's bulletin board: its newest posts, the
comments under them, and whether the caller may post.

Every participant sees the board — the owner, professors, monitors and enrolled
students. Only the staff may start a post, which is what the `can_post` flag
reports; everybody who can see it may comment.
*/
func ListOfferingBulletin(
	ctx context.Context, offeringID int64, claims map[string]any,
) (*models.BulletinBoard, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanView() {
		return nil, ErrOfferingNotFound
	}

	board := &models.BulletinBoard{
		OfferingID: offeringID,
		CanPost:    rel.CanPost(),
		Posts:      []models.BulletinPost{},
	}

	if err := database.DB.QueryRowContext(ctx,
		`SELECT count(*) FROM offering_posts WHERE offering_id = $1`, offeringID,
	).Scan(&board.PostCount); err != nil {
		slog.ErrorContext(ctx, "error counting bulletin posts",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	rows, err := database.DB.QueryContext(ctx, bulletinPostQuery,
		offeringID, bulletinBoardPostLimit,
	)
	if err != nil {
		slog.ErrorContext(ctx, "error listing bulletin posts",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	// The posts are kept in the order the query returned them (newest first) and
	// indexed by id so the comment pass can attach each one without searching.
	byID := make(map[int64]int, bulletinBoardPostLimit)

	for rows.Next() {
		var post models.BulletinPost
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Body, &post.CreatedAt,
			&post.AuthorID, &post.AuthorName, &post.AuthorRole,
		); err != nil {
			slog.ErrorContext(ctx, "error reading a bulletin post",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		post.Comments = []models.BulletinComment{}
		byID[post.ID] = len(board.Posts)
		board.Posts = append(board.Posts, post)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating bulletin posts",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	if len(board.Posts) == 0 {
		return board, nil
	}

	ids := make([]int64, 0, len(board.Posts))
	for _, post := range board.Posts {
		ids = append(ids, post.ID)
	}

	commentRows, err := database.DB.QueryContext(ctx, bulletinCommentQuery, pq.Array(ids))
	if err != nil {
		slog.ErrorContext(ctx, "error listing bulletin comments",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer commentRows.Close()

	for commentRows.Next() {
		var (
			postID  int64
			comment models.BulletinComment
			parent  sql.NullInt64
		)
		if err := commentRows.Scan(
			&postID, &comment.ID, &parent, &comment.Body, &comment.CreatedAt,
			&comment.AuthorID, &comment.AuthorName, &comment.AuthorRole,
		); err != nil {
			slog.ErrorContext(ctx, "error reading a bulletin comment",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}

		if parent.Valid {
			parentID := parent.Int64
			comment.ParentCommentID = &parentID
		}

		if index, ok := byID[postID]; ok {
			board.Posts[index].Comments = append(board.Posts[index].Comments, comment)
		}
	}
	if err := commentRows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating bulletin comments",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return board, nil
}

/*
CreateOfferingPost publishes an announcement on a class's bulletin board.

Only the staff writes posts: the owner, or a professor or monitor assigned to the
class. Every enrolled student is then emailed the post itself, so nobody has to
sign in just to read the announcement.
*/
func CreateOfferingPost(
	ctx context.Context, offeringID int64, claims map[string]any,
	req *models.CreateBulletinPostRequest,
) (*models.BulletinPost, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanView() {
		return nil, ErrOfferingNotFound
	}
	if !rel.CanPost() {
		return nil, ErrBulletinPostForbidden
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if err := validation.ValidateRequiredString(req.Title, MaxBulletinPostTitleLength); err != nil {
		return nil, err
	}
	if err := validation.ValidateRequiredString(req.Body, MaxBulletinPostBodyLength); err != nil {
		return nil, err
	}

	post := models.BulletinPost{
		AuthorID:   userID,
		AuthorName: claimName(claims),
		AuthorRole: relationRole(rel),
		Title:      req.Title,
		Body:       req.Body,
		Comments:   []models.BulletinComment{},
	}

	if err := database.DB.QueryRowContext(ctx, `
		INSERT INTO offering_posts (offering_id, author_id, title, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		offeringID, userID, post.Title, post.Body,
	).Scan(&post.ID, &post.CreatedAt); err != nil {
		slog.ErrorContext(ctx, "error creating a bulletin post",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "bulletin post published",
		slog.Int64("offering_id", offeringID),
		slog.Int64("post_id", post.ID),
		slog.Int64("author_id", userID),
	)

	go publishPostNotification(offeringID, rel.Offering.Name, post)

	return &post, nil
}

/*
CreateBulletinComment adds a comment to a post.

Any participant may comment: enrolled students, monitors and professors. The
comment can answer another comment of the same post, which is what routes the
notification to that comment's author instead of to the class staff.
*/
func CreateBulletinComment(
	ctx context.Context, offeringID, postID int64, claims map[string]any,
	req *models.CreateBulletinCommentRequest,
) (*models.BulletinComment, error) {
	userID, role, err := claimsUserID(claims)
	if err != nil {
		return nil, err
	}

	rel, err := loadOfferingRelation(ctx, offeringID, userID, role)
	if err != nil {
		return nil, err
	}
	if !rel.CanView() {
		return nil, ErrOfferingNotFound
	}

	// The post must belong to the class in the path, so the offering id in the
	// URL cannot be used to comment on another class's post.
	var postTitle string
	err = database.DB.QueryRowContext(ctx, `
		SELECT title FROM offering_posts
		WHERE id = $1 AND offering_id = $2`, postID, offeringID,
	).Scan(&postTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBulletinPostNotFound
	} else if err != nil {
		slog.ErrorContext(ctx, "error loading a bulletin post",
			slog.Int64("post_id", postID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	req.Body = strings.TrimSpace(req.Body)
	if err := validation.ValidateRequiredString(req.Body, MaxBulletinCommentLength); err != nil {
		return nil, err
	}

	// The comment being answered must be part of the same post, and its author
	// (with their class role) decides who is notified.
	var parent bulletinParticipant
	if req.ParentCommentID != nil {
		parent, err = loadBulletinParticipant(ctx, `
			SELECT u.id, u.name, u.email,
			       CASE WHEN o.owner_id = u.id THEN 'professor'
			            ELSE COALESCE(en.role::text, 'student') END,
			       COALESCE(en.banned, FALSE)
			FROM offering_post_comments c
			JOIN offering_posts p ON p.id = c.post_id
			JOIN offerings o ON o.id = p.offering_id
			JOIN users u ON u.id = c.author_id
			LEFT JOIN enrollments en
			       ON en.offering_id = p.offering_id AND en.user_id = c.author_id
			WHERE c.id = $1 AND c.post_id = $2`,
			*req.ParentCommentID, postID,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBulletinCommentNotFound
		} else if err != nil {
			slog.ErrorContext(ctx, "error loading the comment being answered",
				slog.Int64("comment_id", *req.ParentCommentID),
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
	}

	comment := models.BulletinComment{
		ParentCommentID: req.ParentCommentID,
		AuthorID:        userID,
		AuthorName:      claimName(claims),
		AuthorRole:      relationRole(rel),
		Body:            req.Body,
	}

	if err := database.DB.QueryRowContext(ctx, `
		INSERT INTO offering_post_comments (post_id, parent_comment_id, author_id, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		postID, req.ParentCommentID, userID, comment.Body,
	).Scan(&comment.ID, &comment.CreatedAt); err != nil {
		slog.ErrorContext(ctx, "error creating a bulletin comment",
			slog.Int64("post_id", postID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	slog.InfoContext(ctx, "bulletin comment published",
		slog.Int64("offering_id", offeringID),
		slog.Int64("post_id", postID),
		slog.Int64("comment_id", comment.ID),
		slog.Int64("author_id", userID),
	)

	go publishCommentNotification(offeringID, rel.Offering.Name, postTitle, comment, parent)

	return &comment, nil
}

// bulletinPostQuery is the board listing: newest posts first, with the author's
// name and their role *in this class* (the owner is a professor even without an
// enrollment row).
const bulletinPostQuery = `
	SELECT p.id, p.title, p.body, p.created_at, u.id, u.name,
	       CASE WHEN o.owner_id = u.id THEN 'professor'
	            ELSE COALESCE(en.role::text, 'student') END
	FROM offering_posts p
	JOIN users u ON u.id = p.author_id
	JOIN offerings o ON o.id = p.offering_id
	LEFT JOIN enrollments en
	       ON en.offering_id = p.offering_id AND en.user_id = p.author_id
	WHERE p.offering_id = $1
	ORDER BY p.created_at DESC, p.id DESC
	LIMIT $2`

// bulletinCommentQuery reads the discussion of the posts in the listing in one
// query, oldest first.
const bulletinCommentQuery = `
	SELECT c.post_id, c.id, c.parent_comment_id, c.body, c.created_at,
	       u.id, u.name,
	       CASE WHEN o.owner_id = u.id THEN 'professor'
	            ELSE COALESCE(en.role::text, 'student') END
	FROM offering_post_comments c
	JOIN offering_posts p ON p.id = c.post_id
	JOIN offerings o ON o.id = p.offering_id
	JOIN users u ON u.id = c.author_id
	LEFT JOIN enrollments en
	       ON en.offering_id = p.offering_id AND en.user_id = c.author_id
	WHERE c.post_id = ANY($1)
	ORDER BY c.created_at, c.id`

// claimName is the display name the session claims carry.
func claimName(claims map[string]any) string {
	name, _ := claims["name"].(string)
	return name
}

// relationRole is the caller's role *in the class*, for the badge next to their
// name. The owner is a professor even when they have no enrollment row.
func relationRole(rel *offeringRelation) string {
	if rel.Owner || rel.AdminOverride {
		return EnrollmentRoleProfessor
	}
	if rel.Role == "" {
		return "student"
	}

	return rel.Role
}

// --- notification -----------------------------------------------------------

/*
publishPostNotification and publishCommentNotification are the side effects that
email the class. They are variables so a test can observe the notification
without an SMTP server or a second database round trip.

Both are called in their own goroutine: the post is already stored, and a slow
relay must not delay the request that published it.
*/
var (
	publishPostNotification    = notifyBulletinPost
	publishCommentNotification = notifyBulletinComment
)

// emailRecipient is one address a bulletin notification goes to.
type emailRecipient struct {
	ID    int64
	Name  string
	Email string
}

// bulletinAudience selects which members of a class a notification reaches.
type bulletinAudience int

const (
	// bulletinAudienceStudents is every enrolled, non-banned student.
	bulletinAudienceStudents bulletinAudience = iota
	// bulletinAudienceStaff is the owner plus every professor and monitor of the
	// class.
	bulletinAudienceStaff
)

// bulletinParticipant is a comment's author together with their class role and
// whether the class banned them.
type bulletinParticipant struct {
	ID     int64
	Name   string
	Email  string
	Role   string
	Banned bool
}

/*
bulletinLink is the browser address of the class page, where the board lives. It
is empty when the deployment has no public URL (a local instance without links in
its mail).
*/
func bulletinLink(offeringID int64) string {
	if base := config.Get().PublicURL; base != "" {
		return fmt.Sprintf("%s/classes/%d", base, offeringID)
	}

	return ""
}

/*
notifyBulletinPost emails the class's students about a new post. The message
carries the post itself, so a student does not have to sign in to read the
announcement.

The send runs in the background: the post is already stored, and a slow SMTP
relay must not delay the request that published it.
*/
func notifyBulletinPost(offeringID int64, offeringName string, post models.BulletinPost) {
	ctx, cancel := context.WithTimeout(context.Background(), bulletinEmailTimeout)
	defer cancel()

	recipients, err := loadBulletinRecipients(ctx, offeringID, bulletinAudienceStudents)
	if err != nil {
		return
	}

	subject, text, htmlBody := bulletinPostEmail(offeringID, offeringName, post)
	deliverBulletinEmails(ctx, recipients, subject, text, htmlBody)
}

/*
notifyBulletinComment emails whoever has to read a new comment.

A reply to a student's comment reaches only that student: they asked a question
and the answer is for them. Anything else — a new thread on a post, or an answer
to a staff member — reaches the class staff, because the post is theirs to watch.
*/
func notifyBulletinComment(
	offeringID int64,
	offeringName, postTitle string,
	comment models.BulletinComment,
	parent bulletinParticipant,
) {
	ctx, cancel := context.WithTimeout(context.Background(), bulletinEmailTimeout)
	defer cancel()

	var recipients []emailRecipient

	// Only a student still enrolled in the class hears back here: a banned
	// account has lost access to the board, so the answer follows the staff path
	// (which excludes banned enrollments).
	if parent.ID != 0 && parent.Role == EnrollmentRoleStudent && !parent.Banned {
		recipients = []emailRecipient{{
			ID:    parent.ID,
			Name:  parent.Name,
			Email: parent.Email,
		}}
	} else {
		staff, err := loadBulletinRecipients(ctx, offeringID, bulletinAudienceStaff)
		if err != nil {
			return
		}
		recipients = staff
	}

	// Nobody is notified about their own comment.
	recipients = excludeRecipient(recipients, comment.AuthorID)
	if len(recipients) == 0 {
		return
	}

	subject, text, htmlBody := bulletinCommentEmail(
		offeringID, offeringName, postTitle, comment,
	)
	deliverBulletinEmails(ctx, recipients, subject, text, htmlBody)
}

// excludeRecipient drops userID from the list, keeping the order.
func excludeRecipient(recipients []emailRecipient, userID int64) []emailRecipient {
	kept := recipients[:0]
	for _, recipient := range recipients {
		if recipient.ID != userID {
			kept = append(kept, recipient)
		}
	}

	return kept
}

/*
loadBulletinRecipients lists who a notification goes to.

Students are the enrolled role 'student'; the staff is the owner plus every
professor and monitor. Banned enrollments are left out in both cases, so a banned
student stops receiving class mail.
*/
func loadBulletinRecipients(
	ctx context.Context, offeringID int64, audience bulletinAudience,
) ([]emailRecipient, error) {
	query := `
		SELECT DISTINCT u.id, u.name, u.email
		FROM enrollments en
		JOIN users u ON u.id = en.user_id
		WHERE en.offering_id = $1 AND en.role = 'student' AND NOT en.banned
		ORDER BY 2, 3`

	if audience == bulletinAudienceStaff {
		query = `
			SELECT DISTINCT u.id, u.name, u.email
			FROM users u
			JOIN offerings o ON o.id = $1 AND o.owner_id = u.id
			UNION
			SELECT u.id, u.name, u.email
			FROM enrollments en
			JOIN users u ON u.id = en.user_id
			WHERE en.offering_id = $1
			  AND en.role IN ('professor', 'monitor')
			  AND NOT en.banned
			ORDER BY 2, 3`
	}

	rows, err := database.DB.QueryContext(ctx, query, offeringID)
	if err != nil {
		slog.ErrorContext(ctx, "error loading bulletin recipients",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}
	defer rows.Close()

	recipients := make([]emailRecipient, 0, 32)
	for rows.Next() {
		var recipient emailRecipient
		if err := rows.Scan(&recipient.ID, &recipient.Name, &recipient.Email); err != nil {
			slog.ErrorContext(ctx, "error reading a bulletin recipient",
				slog.String("error", err.Error()),
			)
			return nil, ErrServer
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		slog.ErrorContext(ctx, "error iterating bulletin recipients",
			slog.String("error", err.Error()),
		)
		return nil, ErrServer
	}

	return recipients, nil
}

// loadBulletinParticipant reads the single row a reply is notified about.
func loadBulletinParticipant(
	ctx context.Context, query string, args ...any,
) (bulletinParticipant, error) {
	var participant bulletinParticipant

	err := database.DB.QueryRowContext(ctx, query, args...).Scan(
		&participant.ID, &participant.Name, &participant.Email, &participant.Role,
		&participant.Banned,
	)

	return participant, err
}

// deliverBulletinEmails sends one already-rendered message to every recipient.
func deliverBulletinEmails(
	ctx context.Context, recipients []emailRecipient, subject, text, htmlBody string,
) {
	failures := 0

	for _, recipient := range recipients {
		err := sendMail(ctx, mailer.Message{
			To:      recipient.Email,
			Subject: subject,
			Text:    text,
			HTML:    htmlBody,
		})
		if err != nil {
			failures++
			slog.ErrorContext(ctx, "error sending a bulletin email",
				slog.String("recipient", recipient.Email),
				slog.String("error", err.Error()),
			)
		}
	}

	slog.InfoContext(ctx, "bulletin email delivery finished",
		slog.Int("recipients", len(recipients)),
		slog.Int("failures", failures),
	)
}

/*
bulletinPostEmail renders the announcement. It is a pure function so the wording
and the escaping can be tested without a database or an SMTP server.
*/
func bulletinPostEmail(
	offeringID int64, offeringName string, post models.BulletinPost,
) (subject, text, htmlBody string) {
	subject = fmt.Sprintf("[%s] Mural: %s",
		subjectFragment(offeringName), subjectFragment(post.Title),
	)

	link := bulletinLink(offeringID)
	author := subjectFragment(post.AuthorName)

	text = fmt.Sprintf(
		"Olá!\n\n%s publicou no mural da turma %s.\n\n%s\n\n%s\n",
		author, offeringName, post.Title, post.Body,
	)
	if link != "" {
		text += "\nPara comentar, acesse: " + link + "\n"
	}

	htmlBody = fmt.Sprintf(
		"<p>Olá!</p><p><strong>%s</strong> publicou no mural da turma "+
			"<strong>%s</strong>.</p><h3>%s</h3><p>%s</p>",
		html.EscapeString(author),
		html.EscapeString(offeringName),
		html.EscapeString(post.Title),
		nl2br(post.Body),
	)
	if link != "" {
		htmlBody += fmt.Sprintf(
			`<p><a href="%s">Comentar no mural</a></p>`, html.EscapeString(link),
		)
	}

	return subject, text, htmlBody
}

/*
bulletinCommentEmail renders the comment notification. The post title is in the
subject and the body, so the recipient knows which thread it belongs to without
opening the class page.
*/
func bulletinCommentEmail(
	offeringID int64, offeringName, postTitle string, comment models.BulletinComment,
) (subject, text, htmlBody string) {
	subject = fmt.Sprintf("[%s] Novo comentário: %s",
		subjectFragment(offeringName), subjectFragment(postTitle),
	)

	link := bulletinLink(offeringID)
	author := subjectFragment(comment.AuthorName)

	text = fmt.Sprintf(
		"Olá!\n\n%s comentou no mural da turma %s, no post %q.\n\n%s\n",
		author, offeringName, postTitle, comment.Body,
	)
	if link != "" {
		text += "\nPara responder, acesse: " + link + "\n"
	}

	htmlBody = fmt.Sprintf(
		"<p>Olá!</p><p><strong>%s</strong> comentou no post <strong>%s</strong> "+
			"do mural da turma <strong>%s</strong>.</p><blockquote>%s</blockquote>",
		html.EscapeString(author),
		html.EscapeString(postTitle),
		html.EscapeString(offeringName),
		nl2br(comment.Body),
	)
	if link != "" {
		htmlBody += fmt.Sprintf(
			`<p><a href="%s">Responder no mural</a></p>`, html.EscapeString(link),
		)
	}

	return subject, text, htmlBody
}

// subjectFragment collapses whitespace so a title or a name cannot inject a line
// break into a mail header.
func subjectFragment(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// nl2br escapes text for HTML and keeps its line breaks.
func nl2br(value string) string {
	escaped := html.EscapeString(value)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")

	return strings.ReplaceAll(escaped, "\n", "<br>")
}
