package podman

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSocketAdviceStates pins the diagnostics that turn a bare "connection
// refused" into the host-side fix. A path that is not a socket at all means the
// container was created before the socket existed (the bind mount pinned whatever
// was there, a directory Docker created), which is the state a fresh `docker
// compose up` without podman.socket leaves behind.
func TestSocketAdviceStates(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "plain")
	if err := os.WriteFile(filePath, []byte("not a socket"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tests := []struct {
		name     string
		uri      string
		want     string
		wantHint string
	}{
		{
			name:     "missing socket",
			uri:      "unix://" + filepath.Join(dir, "absent.sock"),
			wantHint: "does not exist in this container",
		},
		{
			name:     "a directory where the socket should be",
			uri:      "unix://" + dir,
			wantHint: "is a directory, not a socket",
		},
		{
			name:     "a plain file where the socket should be",
			uri:      "unix://" + filePath,
			wantHint: "is not a socket",
		},
		{
			name: "a remote podman has nothing to diagnose locally",
			uri:  "tcp://podman.example:8888",
			want: "",
		},
		{
			name: "an empty URI falls back to the bindings' own defaults",
			uri:  "",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := socketAdvice(tc.uri)
			if tc.wantHint == "" {
				if got != tc.want {
					t.Fatalf("socketAdvice(%q) = %q, want %q", tc.uri, got, tc.want)
				}
				return
			}

			if !strings.Contains(got, tc.wantHint) {
				t.Fatalf("socketAdvice(%q) = %q, want it to mention %q",
					tc.uri, got, tc.wantHint)
			}
			// Every hint names the command that fixes it: the point of the message
			// is that the reader does not have to work out why it broke.
			if !strings.Contains(got, "docker compose up -d --force-recreate judge") {
				t.Fatalf("socketAdvice(%q) = %q, want the recreate command", tc.uri, got)
			}
		})
	}
}

// TestSocketAdviceOnAStaleSocket covers the second failure mode: the socket is
// there, but it is one nothing listens on any more — a socket recreated after the
// container was, which the mount cannot follow.
func TestSocketAdviceOnAStaleSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "podman.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		// A sandbox that forbids creating unix sockets cannot set this state up.
		t.Skipf("cannot create a unix socket here: %v", err)
	}
	defer listener.Close()

	got := socketAdvice("unix://" + socketPath)
	if !strings.Contains(got, "refuses connections") {
		t.Fatalf("socketAdvice = %q, want it to explain the refused connection", got)
	}
	if !strings.Contains(got, "docker compose up -d --force-recreate judge") {
		t.Fatalf("socketAdvice = %q, want the recreate command", got)
	}
}
