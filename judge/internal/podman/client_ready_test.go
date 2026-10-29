package podman

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.podman.io/podman/v6/version"
)

// fakePodmanAPI serves handler on a unix socket and returns its bindings URI.
func fakePodmanAPI(t *testing.T, handler http.Handler) string {
	t.Helper()

	// A unix socket path is limited to ~100 bytes, so do not build it from
	// t.TempDir(): the test name and the per-test counter make it long.
	dir, err := os.MkdirTemp("", "podman")
	if err != nil {
		t.Fatalf("create socket dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "api.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
		<-done
	})

	return "unix://" + path
}

// TestReadyStaysCheap pins the cost of a readiness probe. Ready must stay on the
// connection handshake (GET /_ping, ~1ms) and never reach for system.Version:
// on the server that endpoint is podman's `podman info` path, which execs conmon
// and the OCI runtime and reads the image and container store. It measured
// ~200ms on a warm host and ~700ms in production, where it dominated /readyz and
// was paid by the blackbox probe and by every submission's pre-flight check.
//
// The fake API answers the ping instantly and stalls anything else, so an
// implementation that reaches for a heavier endpoint fails on both the clock and
// the call count.
func TestReadyStaysCheap(t *testing.T) {
	var expensiveCalls int32
	uri := fakePodmanAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The handshake pings the versioned path, e.g. /v6.1.4/libpod/_ping.
		if strings.HasSuffix(r.URL.Path, "/_ping") {
			w.Header().Set("Libpod-API-Version",
				version.APIVersion[version.Libpod][version.CurrentAPI].String())
			w.WriteHeader(http.StatusOK)
			return
		}

		atomic.AddInt32(&expensiveCalls, 1)
		time.Sleep(time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Components":[]}`))
	}))

	client := New(uri, Limits{})

	start := time.Now()
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("Ready took %v; a readiness probe must not run podman's info path", elapsed)
	}
	if calls := atomic.LoadInt32(&expensiveCalls); calls != 0 {
		t.Fatalf("Ready made %d call(s) past the handshake", calls)
	}
}

// TestReadyReportsUnreachableSocket keeps the failure semantics: a podman that is
// not there has to fail readiness, not report it as ready.
//
// Expect this one to take ~600ms: the bindings retry a failed request three times
// with a 100/200/300ms backoff before giving up, which is what a probe with a
// broken socket costs.
func TestReadyReportsUnreachableSocket(t *testing.T) {
	client := New("unix://"+filepath.Join(t.TempDir(), "absent.sock"), Limits{})

	if err := client.Ready(context.Background()); err == nil {
		t.Fatal("Ready reported ready for a socket that does not exist")
	}
}
