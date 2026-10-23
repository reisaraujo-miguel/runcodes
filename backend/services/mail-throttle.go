package services

import (
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

// allow reports whether a message to key may be sent at now, recording the send
// when it is allowed. An empty key is refused.
func (t *recipientThrottle) allow(key string, now time.Time) bool {
	if key == "" {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.lastSent[key]; ok && now.Sub(last) < t.cooldown {
		return false
	}

	if len(t.lastSent) >= t.max {
		for existing, sentAt := range t.lastSent {
			if now.Sub(sentAt) >= t.cooldown {
				delete(t.lastSent, existing)
			}
		}
		if len(t.lastSent) >= t.max {
			return false
		}
	}

	t.lastSent[key] = now
	return true
}

var transactionalEmailThrottle = newRecipientThrottle(
	recipientCooldown, maxTrackedRecipients,
)

// allowTransactionalEmail normalizes an address and applies the process-wide
// cooldown. It is called by the public email-sending flows.
func allowTransactionalEmail(email string) bool {
	return transactionalEmailThrottle.allow(
		validation.NormalizeEmail(email), time.Now(),
	)
}
