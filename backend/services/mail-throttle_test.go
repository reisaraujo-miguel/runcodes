package services

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecipientThrottle(t *testing.T) {
	throttle := newRecipientThrottle(5*time.Minute, 2)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

	if ok, _ := throttle.reserve("ada@example.com", now); !ok {
		t.Fatal("first send was refused")
	}
	if ok, reason := throttle.reserve("ada@example.com", now.Add(time.Minute)); ok {
		t.Fatal("second send within the cooldown was allowed")
	} else if reason != refusedCoolingDown {
		t.Fatalf("refusal reason = %q, want %q", reason, refusedCoolingDown)
	}
	if ok, _ := throttle.reserve("ada@example.com", now.Add(6*time.Minute)); !ok {
		t.Fatal("send after the cooldown was refused")
	}

	// The map is bounded: a new address is refused when the table is full and
	// no entry is old enough to evict.
	if ok, _ := throttle.reserve("grace@example.com", now.Add(7*time.Minute)); !ok {
		t.Fatal("a distinct address was refused while there was room")
	}
	if ok, _ := throttle.reserve("linus@example.com", now.Add(8*time.Minute)); ok {
		t.Fatal("a new address was allowed after the table filled")
	}
}

// A reservation that is released must be usable again: a send that never
// reached the recipient must not hold the cooldown.
func TestRecipientThrottleReleaseFreesTheSlot(t *testing.T) {
	throttle := newRecipientThrottle(5*time.Minute, 2)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

	if ok, _ := throttle.reserve("ada@example.com", now); !ok {
		t.Fatal("first send was refused")
	}
	throttle.release("ada@example.com")

	if ok, reason := throttle.reserve("ada@example.com", now.Add(time.Second)); !ok {
		t.Fatalf("released slot was refused: %s", reason)
	}
}

func TestReserveTransactionalEmailNormalizes(t *testing.T) {
	// Use a dedicated throttle so the process-wide map cannot make the test
	// order-dependent.
	previous := transactionalEmailThrottle
	transactionalEmailThrottle = newRecipientThrottle(time.Minute, 10)
	t.Cleanup(func() { transactionalEmailThrottle = previous })

	if release, reason := reserveTransactionalEmail(" Ada@Example.COM "); release == nil {
		t.Fatalf("first normalized send was refused: %s", reason)
	}
	if release, _ := reserveTransactionalEmail("ada@example.com"); release != nil {
		t.Fatal("case variant bypassed the cooldown")
	}
}

// A successful send keeps its reservation, so repeats inside the cooldown never
// reach the relay.
func TestSendWithinCooldownSuppressesRepeats(t *testing.T) {
	previous := transactionalEmailThrottle
	transactionalEmailThrottle = newRecipientThrottle(5*time.Minute, 10)
	t.Cleanup(func() { transactionalEmailThrottle = previous })

	calls := 0
	send := func() error {
		calls++
		return nil
	}

	for i := 0; i < 3; i++ {
		if err := sendWithinCooldown(
			context.Background(), flowPasswordReset, 7, "ada@example.com", send,
		); err != nil {
			t.Fatalf("sendWithinCooldown = %v, want nil", err)
		}
	}
	if calls != 1 {
		t.Fatalf("send called %d times, want 1", calls)
	}
}

// A failed send returns the error and frees the slot, so retrying after a relay
// outage is not held back by the cooldown.
func TestSendWithinCooldownReleasesAfterAFailedSend(t *testing.T) {
	previous := transactionalEmailThrottle
	transactionalEmailThrottle = newRecipientThrottle(5*time.Minute, 10)
	t.Cleanup(func() { transactionalEmailThrottle = previous })

	relayErr := errors.New("relay down")
	calls := 0
	send := func() error {
		calls++
		if calls == 1 {
			return relayErr
		}
		return nil
	}

	if err := sendWithinCooldown(
		context.Background(), flowPasswordReset, 7, "ada@example.com", send,
	); !errors.Is(err, relayErr) {
		t.Fatalf("first send error = %v, want %v", err, relayErr)
	}
	if err := sendWithinCooldown(
		context.Background(), flowPasswordReset, 7, "ada@example.com", send,
	); err != nil {
		t.Fatalf("retry after a failed send = %v, want nil", err)
	}
	if calls != 2 {
		t.Fatalf("send called %d times, want 2", calls)
	}
}
