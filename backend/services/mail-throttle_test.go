package services

import (
	"testing"
	"time"
)

func TestRecipientThrottle(t *testing.T) {
	throttle := newRecipientThrottle(5*time.Minute, 2)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

	if !throttle.allow("ada@example.com", now) {
		t.Fatal("first send was refused")
	}
	if throttle.allow("ada@example.com", now.Add(time.Minute)) {
		t.Fatal("second send within the cooldown was allowed")
	}
	if !throttle.allow("ada@example.com", now.Add(6*time.Minute)) {
		t.Fatal("send after the cooldown was refused")
	}

	// The map is bounded: a new address is refused when the table is full and
	// no entry is old enough to evict.
	if !throttle.allow("grace@example.com", now.Add(7*time.Minute)) {
		t.Fatal("a distinct address was refused while there was room")
	}
	if throttle.allow("linus@example.com", now.Add(8*time.Minute)) {
		t.Fatal("a new address was allowed after the table filled")
	}
}

func TestAllowTransactionalEmailNormalizes(t *testing.T) {
	// Use a dedicated throttle so the process-wide map cannot make the test
	// order-dependent.
	previous := transactionalEmailThrottle
	transactionalEmailThrottle = newRecipientThrottle(time.Minute, 10)
	t.Cleanup(func() { transactionalEmailThrottle = previous })

	if !allowTransactionalEmail(" Ada@Example.COM ") {
		t.Fatal("first normalized send was refused")
	}
	if allowTransactionalEmail("ada@example.com") {
		t.Fatal("case variant bypassed the cooldown")
	}
}
