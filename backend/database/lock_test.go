package database

import (
	"context"
	"testing"
)

func TestAdvisoryLockKeyIsStable(t *testing.T) {
	if got, want := advisoryLockKey("limit-alerts"), advisoryLockKey("limit-alerts"); got != want {
		t.Fatalf("advisoryLockKey is not deterministic: %d != %d", got, want)
	}
	if advisoryLockKey("limit-alerts") == advisoryLockKey("output-retention") {
		t.Fatal("different jobs must use different advisory lock keys")
	}
}

// Without a configured database (unit tests and tools) the lock is a no-op and
// the job still runs, so callers do not need a special case.
func TestRunWithBackgroundLockWithoutDatabase(t *testing.T) {
	previous := DB
	DB = nil
	t.Cleanup(func() { DB = previous })

	called := false
	RunWithBackgroundLock(context.Background(), "test-job", func() {
		called = true
	})
	if !called {
		t.Fatal("the job must run when no database is configured")
	}
}
