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
)

const (
	// PasswordResetTokenTTL is how long a recovery link stays usable. It is
	// deliberately shorter than the verification link: resetting a password is
	// higher impact, and a recovery mail is usually acted on immediately.
	PasswordResetTokenTTL = time.Hour

	// passwordResetTokenBytes is the entropy of a recovery token. Only its
	// SHA-256 hash is stored, so a database reader cannot turn a dump into
	// working links.
	passwordResetTokenBytes = 32

	// maxPasswordResetTokenLength rejects obviously malformed input before it
	// is hashed. A real token is 43 base64url characters.
	maxPasswordResetTokenLength = 128
)

/*
RequestPasswordReset emails a recovery link to the account that owns the
address. An unknown address is a silent success: the endpoint is public, and a
different answer would tell the caller whether an email has an account here.

Unconfirmed accounts can request a reset too. Resetting the password does not
confirm the account, so the user still has to open a verification link before
they can sign in.
*/
func RequestPasswordReset(ctx context.Context, email string) error {
	email = validation.NormalizeEmail(email)

	var (
		id   int
		name string
	)
	err := database.DB.QueryRowContext(ctx,
		"SELECT id, name FROM users WHERE email = $1", email,
	).Scan(&id, &name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		slog.ErrorContext(ctx, "error looking up account for password reset",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if !allowTransactionalEmail(email) {
		return nil
	}
	return SendPasswordResetEmail(ctx, id, name, email)
}

/*
SendPasswordResetEmail issues a fresh recovery token and emails the link to the
user. Issuing a token invalidates any previous one for that account, so the
newest message is always the one that works.

The token itself is never stored: only its SHA-256 hash is, and the plaintext
exists only in the message body.
*/
func SendPasswordResetEmail(
	ctx context.Context, userID int, name, email string,
) error {
	token, err := issuePasswordResetToken(ctx, userID)
	if err != nil {
		return err
	}

	link := passwordResetLink(token)
	subject, text, htmlBody := passwordResetEmailContent(name, link)

	if err := sendMail(ctx, mailer.Message{
		To:      email,
		Subject: subject,
		Text:    text,
		HTML:    htmlBody,
	}); err != nil {
		return fmt.Errorf("sending password reset email: %w", err)
	}

	slog.InfoContext(ctx, "password reset email sent",
		slog.Int("user_id", userID),
	)
	return nil
}

/*
ResetPassword consumes a recovery token and replaces the account's password.

The token is single use and short lived: a second use is rejected instead of
being treated as success, so a leaked link cannot be replayed. The password is
hashed before the transaction starts, so a hashing failure leaves the token
unconsumed.
*/
func ResetPassword(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxPasswordResetTokenLength {
		return ErrInvalidPasswordResetToken
	}

	hashed, err := hashPassword(newPassword)
	if err != nil {
		slog.ErrorContext(ctx, "error hashing password",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	hash := passwordResetTokenHash(token)

	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing password reset transaction",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}
	defer tx.Rollback()

	var userID int
	err = tx.QueryRowContext(ctx, `
		UPDATE password_reset_tokens
		SET consumed_at = NOW()
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id`, hash,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return ErrInvalidPasswordResetToken
		}
		slog.ErrorContext(ctx, "error consuming password reset token",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, token_version = token_version + 1, updated_at = NOW()
		WHERE id = $2`,
		hashed, userID,
	); err != nil {
		slog.ErrorContext(ctx, "error updating password after reset",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(ctx, "error committing password reset",
			slog.String("error", err.Error()),
		)
		return ErrServer
	}

	slog.InfoContext(ctx, "password reset", slog.Int("user_id", userID))
	return nil
}

/*
issuePasswordResetToken creates a token, stores its hash and returns the
plaintext for the email. The upsert replaces the account's previous token, so
exactly one recovery link is live per account.
*/
func issuePasswordResetToken(ctx context.Context, userID int) (string, error) {
	raw := make([]byte, passwordResetTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		slog.ErrorContext(ctx, "error generating password reset token",
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := passwordResetTokenHash(token)

	if _, err := database.DB.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = EXCLUDED.token_hash,
		    expires_at = EXCLUDED.expires_at,
		    consumed_at = NULL,
		    created_at = NOW()`,
		userID, hash, nowFunc().Add(PasswordResetTokenTTL),
	); err != nil {
		slog.ErrorContext(ctx, "error storing password reset token",
			slog.Int("user_id", userID),
			slog.String("error", err.Error()),
		)
		return "", ErrServer
	}

	return token, nil
}

// passwordResetTokenHash is the one-way value stored in the database.
func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// passwordResetLink points at the SPA route that posts the token to the API.
func passwordResetLink(token string) string {
	return config.Get().PublicURL + "/reset-password?token=" + url.QueryEscape(token)
}

func passwordResetEmailContent(name, link string) (subject, text, htmlBody string) {
	subject = "Redefina sua senha no RunCodes"

	greeting := "Olá!"
	if displayName := strings.TrimSpace(name); displayName != "" {
		greeting = "Olá, " + displayName + "!"
	}

	text = fmt.Sprintf(`%s

Recebemos um pedido para redefinir a senha da sua conta no RunCodes.

Acesse o link para escolher uma nova senha:

%s

O link expira em %d hora e só pode ser usado uma vez. Se você não pediu a
redefinição, ignore este email: a sua senha atual continua válida.

RunCodes
`, greeting, link, int(PasswordResetTokenTTL.Hours()))

	escapedGreeting := html.EscapeString(greeting)
	escapedLink := html.EscapeString(link)
	htmlBody = fmt.Sprintf(`<!doctype html>
<html lang="pt-BR">
<body style="font-family: system-ui, sans-serif; color: #1f2937; line-height: 1.6; max-width: 560px; margin: 0 auto; padding: 24px;">
  <p>%s</p>
  <p>Recebemos um pedido para redefinir a senha da sua conta no RunCodes.</p>
  <p>
    <a href="%s" style="display: inline-block; background: #111827; color: #ffffff; padding: 12px 20px; border-radius: 8px; text-decoration: none;">Redefinir senha</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">Se o botão não funcionar, copie este endereço no navegador:<br>
    <a href="%s">%s</a>
  </p>
  <p style="font-size: 14px; color: #4b5563;">O link expira em %d hora e só pode ser usado uma vez. Se você não pediu a redefinição, ignore este email: a sua senha atual continua válida.</p>
</body>
</html>
`, escapedGreeting, escapedLink, escapedLink, escapedLink, int(PasswordResetTokenTTL.Hours()))

	return subject, text, htmlBody
}
