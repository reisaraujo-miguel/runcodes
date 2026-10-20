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
ResendVerification sends a new verification link to an account that is still
unconfirmed.

An unknown address and an already confirmed account are both a silent success:
the endpoint is public, and answering differently would turn it into a way to
ask whether an email has an account here.
*/
func ResendVerification(ctx context.Context, email string) error {
	var (
		id        int
		name      string
		confirmed bool
	)
	err := database.DB.QueryRowContext(ctx, `
		SELECT id, name, confirmed FROM users WHERE email = $1`,
		email,
	).Scan(&id, &name, &confirmed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		slog.ErrorContext(ctx, "error looking up account for verification resend",
			slog.String("error", err.Error()),
		)
		return ErrServer
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

	var userID int
	err = tx.QueryRowContext(ctx, `
		UPDATE email_verification_tokens
		SET consumed_at = NOW()
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id`, hash,
	).Scan(&userID)
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

	if _, err := tx.ExecContext(ctx, `
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

	slog.InfoContext(ctx, "email verified", slog.Int("user_id", userID))
	return nil
}

/*
IsEmailVerified reports whether the account has confirmed its email. It backs the
route guard that keeps a session issued before a confirmation was revoked (an
admin unchecking the account, say) from continuing to act.
*/
func IsEmailVerified(ctx context.Context, userID int) (bool, error) {
	var confirmed bool
	err := database.DB.QueryRowContext(ctx,
		"SELECT confirmed FROM users WHERE id = $1", userID,
	).Scan(&confirmed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrUserNotFound
		}
		slog.ErrorContext(ctx, "error checking email verification",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return false, ErrServer
	}
	return confirmed, nil
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
// success only when the token is known and its account is already confirmed.
func verificationLinkReuse(ctx context.Context, hash string) error {
	var confirmed bool
	err := database.DB.QueryRowContext(ctx, `
		SELECT u.confirmed
		FROM email_verification_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1`, hash,
	).Scan(&confirmed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidVerificationToken
		}
		slog.ErrorContext(ctx, "error resolving verification token",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if confirmed {
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
