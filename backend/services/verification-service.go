package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/database"
	"github.com/runcodes-icmc/runcodes/mailer"
	"github.com/runcodes-icmc/runcodes/validation"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

const (
	// VerificationTokenTTL is how long a verification link stays usable. It is
	// long enough to survive an evening inbox and short enough that a leaked old
	// message is not a permanent key.
	VerificationTokenTTL = 24 * time.Hour

	// verificationTokenBytes is the entropy of a link token. Only its SHA-256
	// hash is stored, so a database reader cannot turn a dump into working links.
	verificationTokenBytes = 32

	// maxVerificationTokenLength rejects obviously malformed input before it is
	// hashed. A real token is 43 base64url characters.
	maxVerificationTokenLength = 128
)

// sendMail is a variable so tests can capture the message instead of talking to
// an SMTP server.
var sendMail = mailer.Send

/*
SendVerificationEmail issues a fresh verification token and emails the link to
the user. Issuing a token invalidates any previous one for that user, so the
newest message is always the one that works.

The token itself is never stored: only its SHA-256 hash is, and the plaintext
exists only in the message body.
*/
func SendVerificationEmail(
	ctx context.Context, userID int, name, email string,
) error {
	token, err := issueVerificationToken(ctx, userID)
	if err != nil {
		return err
	}

	link := verificationLink(token)
	subject, text, htmlBody := verificationEmailContent(name, link)

	if err := sendMail(ctx, mailer.Message{
		To:      email,
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}); err != nil {
		return fmt.Errorf("sending verification email: %w", err)
	}

	slog.InfoContext(ctx, "account verification email sent",
		slog.Int("user_id", userID),
	)
	return nil
}

/*
SendEmailChangeEmail issues a verification token for a requested new address and
emails the link there. The current address is not changed until the link is
used, so a typo or an undeliverable address cannot lock the owner out.
*/
func SendEmailChangeEmail(
	ctx context.Context, userID int, name, newEmail string,
) error {
	token, err := issueVerificationToken(ctx, userID)
	if err != nil {
		return err
	}

	link := verificationLink(token)
	subject, text, htmlBody := emailChangeEmailContent(name, link)

	if err := sendMail(ctx, mailer.Message{
		To:      newEmail,
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}); err != nil {
		return fmt.Errorf("sending email change verification: %w", err)
	}

	slog.InfoContext(ctx, "email change verification sent",
		slog.Int("user_id", userID),
	)
	return nil
}

/*
ResendVerification sends a new verification link to an account that is still
unconfirmed, or re-sends the pending email-change link when one is in flight. A
pending change must keep its token addressed to the new address: sending an
account-verification link to the old address would let that link complete the
change without proving control of the new one.

An unknown address and an already confirmed account with no pending change are
both a silent success: the endpoint is public, and answering differently would
turn it into a way to ask whether an email has an account here.
*/
func ResendVerification(ctx context.Context, email string) error {
	email = validation.NormalizeEmail(email)

	var (
		id          int
		name        string
		confirmed   bool
		pendingMail string
	)
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, confirmed, COALESCE(pending_email, '')
		FROM users WHERE email = $1`,
		email,
	).Scan(&id, &name, &confirmed, &pendingMail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		slog.ErrorContext(ctx, "error looking up account for verification resend",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if pendingMail != "" {
		return SendEmailChangeEmail(ctx, id, name, pendingMail)
	}
	if confirmed {
		return nil
	}

	return SendVerificationEmail(ctx, id, name, email)
}

/*
VerifyEmail consumes a verification token and marks the account confirmed.

The token is single use. Reusing a link after a successful verification is
answered with success as well, so a reload, a double click or an email client
that prefetches the link does not show a spurious error; a token that is unknown,
expired, or belongs to an account an admin later unconfirmed is invalid.
*/
func VerifyEmail(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxVerificationTokenLength {
		return ErrInvalidVerificationToken
	}
	hash := verificationTokenHash(token)

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing verification transaction",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	defer tx.Rollback()

	var (
		userID      int
		pendingMail string
	)
	err = tx.QueryRowContext(ctx, `
		UPDATE email_verification_tokens t
		SET consumed_at = NOW()
		FROM users u
		WHERE t.token_hash = $1
		  AND t.consumed_at IS NULL
		  AND t.expires_at > NOW()
		  AND u.id = t.user_id
		RETURNING t.user_id, COALESCE(u.pending_email, '')`, hash,
	).Scan(&userID, &pendingMail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return verificationLinkReuse(ctx, hash)
		}
		slog.ErrorContext(ctx, "error consuming verification token",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if pendingMail != "" {
		// This token verifies an email change, not account creation: only now
		// does the advertised address become the new one. Bumping token_version
		// signs every existing session out, including a stolen one.
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET email = $1, pending_email = NULL, confirmed = TRUE,
			    token_version = token_version + 1, updated_at = NOW()
			WHERE id = $2`,
			pendingMail, userID,
		); err != nil {
			if pgErr, ok := err.(*pq.Error); ok &&
				pgErr.Code == pqerror.UniqueViolation {
				return ErrEmailExists
			}
			slog.ErrorContext(ctx, "error applying email change",
				slog.Int("user_id", userID),
				slog.String("error", err.Error()),
			)
			return ErrServer
		}
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE users SET confirmed = TRUE, updated_at = NOW() WHERE id = $1`,
		userID,
	); err != nil {
		slog.ErrorContext(ctx, "error confirming account",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing verification",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	slog.InfoContext(ctx, "email verified",
		slog.Int("user_id", userID),
		slog.Bool("email_changed", pendingMail != ""),
	)
	return nil
}

// SessionState is the database-side state the request guard compares with the
// claims of a session token.
type SessionState struct {
	Confirmed    bool
	Role         string
	TokenVersion int
}

/*
GetSessionState loads the state that backs a session. It is one primary-key
lookup per protected request, next to the queries the handlers already run. It
lets the guard reject a token whose account was unconfirmed, demoted, banned or
had its password/email changed since the token was issued.
*/
func GetSessionState(ctx context.Context, userID int) (SessionState, error) {
	var state SessionState
	err := database.DB.QueryRowContext(ctx, `
		SELECT confirmed, role::text, token_version
		FROM users WHERE id = $1`, userID,
	).Scan(&state.Confirmed, &state.Role, &state.TokenVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionState{}, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error checking account session state",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return SessionState{}, ErrServer
	}

	return state, nil
}

/*
issueVerificationToken creates a token, stores its hash and returns the
plaintext for the email. The upsert replaces the account's previous token, so
exactly one link is live per account and the newest message is the one that
works.
*/
func issueVerificationToken(ctx context.Context, userID int) (string, error) {
	raw := make([]byte, verificationTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		slog.ErrorContext(ctx, "error generating verification token",
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := verificationTokenHash(token)

	if _, err := database.DB.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = EXCLUDED.token_hash,
		    expires_at = EXCLUDED.expires_at,
		    consumed_at = NULL,
		    created_at = NOW()`,
		userID, hash, nowFunc().Add(VerificationTokenTTL),
	); err != nil {
		slog.ErrorContext(ctx, "error storing verification token",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}

	return token, nil
}

// verificationLinkReuse answers a token that could not be consumed now. It is a
// success only when the token is known, its account is already confirmed, and no
// email change is still pending (a pending change means the first click did not
// complete, so silently reporting success would be wrong).
func verificationLinkReuse(ctx context.Context, hash string) error {
	var (
		confirmed   bool
		pendingMail string
	)
	err := database.DB.QueryRowContext(ctx, `
		SELECT u.confirmed, COALESCE(u.pending_email, '')
		FROM email_verification_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1`, hash,
	).Scan(&confirmed, &pendingMail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidVerificationToken
		}
		slog.ErrorContext(ctx, "error resolving verification token",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if confirmed && pendingMail == "" {
		return nil
	}
	return ErrInvalidVerificationToken
}

// verificationTokenHash is the one-way value stored in the database.
func verificationTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// verificationLink points at the SPA route that posts the token to the API.
func verificationLink(token string) string {
	return config.Get().PublicURL + "/verify-email?token=" + url.QueryEscape(token)
}

func verificationEmailContent(name, link string) (subject, text, htmlBody string) {
	subject = "Confirme seu email no RunCodes"

	greeting := "Olá!"
	if displayName := strings.TrimSpace(name); displayName != "" {
		greeting = "Olá, " + displayName + "!"
	}

	text = fmt.Sprintf(`%s

Confirme o seu email para ativar a sua conta no RunCodes:

%s

O link expira em %d horas. Se você não criou esta conta, ignore este email.

RunCodes
`, greeting, link, int(VerificationTokenTTL.Hours()))

	escapedGreeting := html.EscapeString(greeting)
	escapedLink := html.EscapeString(link)
	htmlBody = fmt.Sprintf(`<!doctype html>
<html lang="pt-BR">
<body style="font-family: system-ui, sans-serif; color: #1f2937; line-height: 1.6; max-width: 560px; margin: 0 auto; padding: 24px;">
  <p>%s</p>
  <p>Confirme o seu email para ativar a sua conta no RunCodes.</p>
  <p>
    <a href="%s" style="display: inline-block; background: #111827; color: #ffffff; padding: 12px 20px; border-radius: 8px; text-decoration: none;">Confirmar email</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">Se o botão não funcionar, copie este endereço no navegador:<br>
    <a href="%s">%s</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">O link expira em %d horas. Se você não criou esta conta, ignore este email.</p>
</body>
</html>
`, escapedGreeting, escapedLink, escapedLink, escapedLink, int(VerificationTokenTTL.Hours()))

	return subject, text, htmlBody
}

func emailChangeEmailContent(name, link string) (subject, text, htmlBody string) {
	subject = "Confirme o novo email da sua conta RunCodes"

	greeting := "Olá!"
	if displayName := strings.TrimSpace(name); displayName != "" {
		greeting = "Olá, " + displayName + "!"
	}

	text = fmt.Sprintf(`%s

Recebemos um pedido para alterar o email da sua conta no RunCodes.

Confirme o novo endereço neste link:

%s

O link expira em %d horas. Até a confirmação, o email atual continua ativo.
Se você não pediu esta alteração, ignore este email.

RunCodes
`, greeting, link, int(VerificationTokenTTL.Hours()))

	escapedGreeting := html.EscapeString(greeting)
	escapedLink := html.EscapeString(link)
	htmlBody = fmt.Sprintf(`<!doctype html>
<html lang="pt-BR">
<body style="font-family: system-ui, sans-serif; color: #1f2937; line-height: 1.6; max-width: 560px; margin: 0 auto; padding: 24px;">
  <p>%s</p>
  <p>Recebemos um pedido para alterar o email da sua conta no RunCodes.</p>
  <p>
    <a href="%s" style="display: inline-block; background: #111827; color: #ffffff; padding: 12px 20px; border-radius: 8px; text-decoration: none;">Confirmar novo email</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">Se o botão não funcionar, copie este endereço no navegador:<br>
    <a href="%s">%s</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">O link expira em %d horas. Até a confirmação, o email atual continua ativo. Se você não pediu esta alteração, ignore este email.</p>
</body>
</html>
`, escapedGreeting, escapedLink, escapedLink, escapedLink, int(VerificationTokenTTL.Hours()))

	return subject, text, htmlBody
}
