package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/runcodes-icmc/runcodes/validation"
)

const (
	// recipientCooldown is the minimum interval between transactional emails to
	// the same address. It protects the recipient from mail bombing by a caller
	// that can reach the public resend/forgot-password endpoints from many IPs.
	recipientCooldown = 5 * time.Minute

	// maxTrackedRecipients bounds the in-memory throttle. When the map is full,
	// expired entries are evicted first; if none are expired, new recipients are
	// refused rather than letting an attacker grow the map without bound.
	maxTrackedRecipients = 10_000
)

// Reasons a reservation is refused. They reach the operator log only: the
// public endpoints answer the same way for every one of them.
const (
	refusedEmpty       = "empty_address"
	refusedCoolingDown = "cooldown"
	refusedTableFull   = "table_full"
)

// Flows that send transactional email, used as the flow attribute in logs.
const (
	flowPasswordReset      = "password_reset"
	flowVerificationResend = "verification_resend"
	flowEmailChange        = "email_change"
)

/*
recipientThrottle is a small per-process cooldown map. A distributed deployment
would move this to Redis, but the current backend is a single process and this
is enough to stop repeated sends to one address.
*/
type recipientThrottle struct {
	mu       sync.Mutex
	lastSent map[string]time.Time
	cooldown time.Duration
	max      int
}

func newRecipientThrottle(cooldown time.Duration, max int) *recipientThrottle {
	return &recipientThrottle{
		lastSent: make(map[string]time.Time),
		cooldown: cooldown,
		max:      max,
	}
}

/*
reserve reports whether a message to key may be sent at now and, when it may,
records the send. The reason is empty on success and names the refusal
otherwise.

A reservation has to be released when the send then fails, so a relay outage
does not lock every recipient out of the next attempt; a reservation that is
kept is what the cooldown is for.
*/
func (t *recipientThrottle) reserve(key string, now time.Time) (bool, string) {
	if key == "" {
		return false, refusedEmpty
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.lastSent[key]; ok && now.Sub(last) < t.cooldown {
		return false, refusedCoolingDown
	}

	if len(t.lastSent) >= t.max {
		for existing, sentAt := range t.lastSent {
			if now.Sub(sentAt) >= t.cooldown {
				delete(t.lastSent, existing)
			}
		}
		if len(t.lastSent) >= t.max {
			return false, refusedTableFull
		}
	}

	t.lastSent[key] = now
	return true, ""
}

// release frees a reservation whose message was not delivered. It is harmless
// when the key holds no reservation.
func (t *recipientThrottle) release(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.lastSent, key)
}

var transactionalEmailThrottle = newRecipientThrottle(
	recipientCooldown, maxTrackedRecipients,
)

/*
reserveTransactionalEmail reserves the cooldown slot for a transactional email to
email. The returned release is nil when the send must be skipped, in which case
reason says why. Normalizing the address first keeps a case variant from
bypassing the cooldown.
*/
func reserveTransactionalEmail(email string) (func(), string) {
	key := validation.NormalizeEmail(email)

	ok, reason := transactionalEmailThrottle.reserve(key, time.Now())
	if !ok {
		return nil, reason
	}

	return func() { transactionalEmailThrottle.release(key) }, ""
}

/*
sendWithinCooldown runs a transactional send under the per-recipient cooldown.

A suppressed send returns nil: the public endpoints must answer the same way
whether or not a message went out, so the log is the only place the difference
is visible. A send that fails releases its reservation and returns the error.
*/
func sendWithinCooldown(
	ctx context.Context, flow string, userID int, email string, send func() error,
) error {
	release, reason := reserveTransactionalEmail(email)
	if release == nil {
		slog.InfoContext(ctx, "transactional email suppressed",
			slog.String("flow", flow),
			slog.Int("user_id", userID),
			slog.String("reason", reason),
		)
		return nil
	}

	if err := send(); err != nil {
		release()
		return err
	}

	return nil
}
