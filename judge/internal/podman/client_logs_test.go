package podman

import (
	"testing"
	"time"
)

// The log request has to carry a `since` bound. Without it podman walks the
// journal from its oldest entry to the live end before delivering anything,
// which is what the `startup` phase used to be made of (see Logs).
func TestLogOptionsBoundTheReplay(t *testing.T) {
	createdAt := time.Date(2026, 11, 2, 12, 36, 58, 0, time.UTC)
	ct := &Container{ID: "abc", createdAt: createdAt}

	options := ct.logOptions()

	if options.Since == nil {
		t.Fatal("the log request carries no since bound")
	}
	since, err := time.Parse(time.RFC3339Nano, *options.Since)
	if err != nil {
		t.Fatalf("since %q is not RFC3339: %v", *options.Since, err)
	}
	want := createdAt.Add(-logReplayMargin)
	if !since.Equal(want) {
		t.Fatalf("since = %s, want %s", since, want)
	}

	if options.Follow == nil || !*options.Follow {
		t.Fatal("the request does not follow the logs")
	}
	if options.Stdout == nil || !*options.Stdout {
		t.Fatal("the request does not ask for stdout")
	}
	if options.Stderr == nil || !*options.Stderr {
		t.Fatal("the request does not ask for stderr")
	}
	// "all" keeps the replay complete inside the since window.
	if options.Tail == nil || *options.Tail != "all" {
		t.Fatalf("tail = %v, want all", options.Tail)
	}
}

// A Container built without Create (tests) still gets a usable bound: the
// request is made right after the container started, so "now" is safe.
func TestLogOptionsWithoutACreationTime(t *testing.T) {
	ct := &Container{ID: "abc"}

	options := ct.logOptions()
	if options.Since == nil {
		t.Fatal("the log request carries no since bound")
	}

	since, err := time.Parse(time.RFC3339Nano, *options.Since)
	if err != nil {
		t.Fatalf("since %q is not RFC3339: %v", *options.Since, err)
	}
	if delta := time.Since(since); delta < time.Second {
		t.Fatalf("since = %s, want roughly now minus the margin", since)
	}
}
