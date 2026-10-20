/*
Package mailer sends transactional email through the configured SMTP server.

It deliberately keeps to the standard library: the platform only needs plain
text and HTML messages (account verification today), and an SMTP client is a
small enough surface to own. A local catcher such as smtp4dev needs no
authentication and no TLS, while a real relay is reached with STARTTLS and
optional PLAIN authentication.

Send is synchronous and bounded by a timeout. Callers that must not fail a
request when mail delivery does (signing up, for example) log the error and
offer a resend path instead.
*/
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

// sendTimeout bounds a single delivery. It stays below the frontend's 15s
// request budget, so a stalled relay fails the send (and is logged) instead of
// making the signup request time out in the browser after the account was
// already created. The SMTP conversation has no context support of its own, so
// the deadline is set on the connection and the context closes it early when
// the caller is cancelled.
const sendTimeout = 10 * time.Second

// Message is one transactional email. HTML may be empty, in which case only the
// text part is sent.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

/*
Send delivers msg through the SMTP endpoint in the configuration.

The connection is always upgraded with STARTTLS when the server advertises it.
Plain SMTP is therefore only used by a local catcher; a production relay that
offers STARTTLS gets an encrypted session without extra configuration.
*/
func Send(ctx context.Context, msg Message) error {
	cfg := config.Get().Mail

	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return fmt.Errorf("mail: invalid sender %q: %w", cfg.From, err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("mail: invalid recipient: %w", err)
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return fmt.Errorf("mail: subject contains a line break")
	}

	address := net.JoinHostPort(cfg.Host, cfg.Port)
	dialer := &net.Dialer{Timeout: sendTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("mail: connecting to %s: %w", address, err)
	}
	defer conn.Close()

	// Closing the socket unblocks a pending SMTP read/write when the request is
	// cancelled or its deadline passes.
	deadline := time.Now().Add(sendTimeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("mail: setting deadline: %w", err)
	}
	stop := make(chan struct{})
	defer close(stop)

	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("mail: starting SMTP session: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{
			ServerName: cfg.Host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return fmt.Errorf("mail: starting TLS: %w", err)
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mail: authenticating: %w", err)
		}
	}

	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mail: sender rejected: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mail: recipient rejected: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: starting message: %w", err)
	}

	body, err := buildMessage(from, to, msg)
	if err != nil {
		_ = writer.Close()
		return err
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("mail: writing message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mail: finishing message: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("mail: ending SMTP session: %w", err)
	}

	return nil
}

/*
buildMessage renders the RFC 5322 message. Text and HTML are sent as
multipart/alternative so a client that cannot render HTML still shows the link.
*/
func buildMessage(from, to *mail.Address, msg Message) ([]byte, error) {
	var buf bytes.Buffer

	writeHeader(&buf, "From", from.String())
	writeHeader(&buf, "To", to.String())
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&buf, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Message-ID", messageID(from.Address))

	if msg.HTML == "" {
		writeHeader(&buf, "Content-Type", "text/plain; charset=utf-8")
		writeHeader(&buf, "Content-Transfer-Encoding", "8bit")
		buf.WriteString("\r\n")
		buf.WriteString(normalizeCRLF(msg.Text))
		return buf.Bytes(), nil
	}

	mw := multipart.NewWriter(&buf)
	_ = mw.SetBoundary(randomBoundary())
	writeHeader(&buf, "Content-Type",
		"multipart/alternative; boundary="+mw.Boundary())
	buf.WriteString("\r\n")

	textPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, fmt.Errorf("mail: creating text part: %w", err)
	}
	if _, err := textPart.Write([]byte(normalizeCRLF(msg.Text))); err != nil {
		return nil, fmt.Errorf("mail: writing text part: %w", err)
	}

	htmlPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {"text/html; charset=utf-8"},
		"Content-Transfer-Encoding": {"8bit"},
	})
	if err != nil {
		return nil, fmt.Errorf("mail: creating HTML part: %w", err)
	}
	if _, err := htmlPart.Write([]byte(normalizeCRLF(msg.HTML))); err != nil {
		return nil, fmt.Errorf("mail: writing HTML part: %w", err)
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("mail: closing multipart message: %w", err)
	}

	return buf.Bytes(), nil
}

func writeHeader(buf *bytes.Buffer, name, value string) {
	fmt.Fprintf(buf, "%s: %s\r\n", name, value)
}

// normalizeCRLF converts the body's line endings to CRLF, as SMTP requires,
// without doubling the ones already there.
func normalizeCRLF(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	return body
}

func messageID(from string) string {
	domain := "runcodes.local"
	if at := strings.LastIndex(from, "@"); at >= 0 && at+1 < len(from) {
		domain = from[at+1:]
	}
	return fmt.Sprintf("<%s@%s>", randomToken(), domain)
}

func randomBoundary() string {
	return "runcodes-" + randomToken()
}

func randomToken() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand failing is not recoverable and would be a broken system;
		// the message is still well formed with a timestamp-based identifier.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
