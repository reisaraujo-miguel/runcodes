// Package podman wraps the podman v6 REST bindings: connection, image
// provisioning and the container lifecycle used by the runner.
package podman

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/specgen"
)

// Client talks to a rootless podman service over its socket.
type Client struct {
	uri     string
	mu      sync.Mutex
	ensured map[string]bool
}

// New creates a client for the given connection URI (e.g.
// "unix:///run/user/1000/podman/podman.sock"). An empty URI lets the bindings
// fall back to $CONTAINER_HOST or the default socket.
func New(uri string) *Client {
	return &Client{uri: uri, ensured: make(map[string]bool)}
}

// Connect returns a context carrying the podman connection.
func (c *Client) Connect(ctx context.Context) (context.Context, error) {
	return bindings.NewConnection(ctx, c.uri)
}

// Ready reports whether the podman service responds (used by /readyz).
func (c *Client) Ready(ctx context.Context) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connect to podman: %w", err)
	}
	if _, err := system.Version(conn, nil); err != nil {
		return fmt.Errorf("podman version: %w", err)
	}
	return nil
}

// EnsureImage makes sure the image is present locally, pulling it if needed.
func (c *Client) EnsureImage(ctx context.Context, image string) error {
	c.mu.Lock()
	already := c.ensured[image]
	c.mu.Unlock()
	if already {
		return nil
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	exists, err := images.Exists(conn, image, nil)
	if err != nil {
		return fmt.Errorf("check image %s: %w", image, err)
	}
	if !exists {
		if _, err := images.Pull(conn, image, nil); err != nil {
			return fmt.Errorf("pull image %s: %w", image, err)
		}
	}
	c.mu.Lock()
	c.ensured[image] = true
	c.mu.Unlock()
	return nil
}

// RemoveByName force-removes a container by name if it exists. Used before
// creating a run so a container orphaned by a previous crash cannot block the
// new one with a name conflict.
func (c *Client) RemoveByName(ctx context.Context, name string) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	exists, err := containers.Exists(conn, name, nil)
	if err != nil {
		return fmt.Errorf("check container %s: %w", name, err)
	}
	if !exists {
		return nil
	}
	if _, err := containers.Remove(conn, name, new(containers.RemoveOptions).WithForce(true).WithVolumes(true)); err != nil {
		return fmt.Errorf("remove stale container %s: %w", name, err)
	}
	return nil
}

// RunConfig describes a container to create.
type RunConfig struct {
	Name        string
	Image       string
	MountSource string // host directory bind-mounted at /root
	Labels      map[string]string
}

// Container is a created container.
type Container struct {
	ID  string
	cfg RunConfig
	// conn carries the podman connection; request contexts are derived from it.
	conn context.Context
}

// Create creates (but does not start) a container running the image's default
// command with the workspace mounted at /root.
func (c *Client) Create(ctx context.Context, rc RunConfig) (*Container, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}

	s := specgen.NewSpecGenerator(rc.Image, false)
	s.Name = rc.Name
	s.Labels = rc.Labels
	// These namespaces are documented as mandatory; explicit private mode
	// matches what a plain `podman run` would use.
	s.PidNS = specgen.Namespace{NSMode: specgen.Private}
	s.UtsNS = specgen.Namespace{NSMode: specgen.Private}
	s.IpcNS = specgen.Namespace{NSMode: specgen.Private}
	s.Mounts = []spec.Mount{{
		Type:        "bind",
		Source:      rc.MountSource,
		Destination: "/root",
		Options:     []string{"rw"},
	}}

	resp, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	return &Container{ID: resp.ID, cfg: rc, conn: conn}, nil
}

// Start starts the container.
func (ct *Container) Start(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ct.conn, 30*time.Second)
	defer cancel()
	if err := containers.Start(reqCtx, ct.ID, nil); err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	return nil
}

// LogStream yields the container's merged stdout/stderr lines.
type LogStream struct {
	Lines <-chan string
	// Err reports the terminal error of the log request (nil on clean EOF).
	Err <-chan error

	cancel context.CancelFunc
}

// Logs follows the container's logs from the beginning, line-buffered.
func (ct *Container) Logs(ctx context.Context) *LogStream {
	streamCtx, cancel := context.WithCancel(ct.conn)

	stdout := make(chan string, 256)
	stderr := make(chan string, 256)
	lines := make(chan string, 256)
	logErr := make(chan error, 1)

	go func() {
		defer close(stdout)
		defer close(stderr)
		logErr <- containers.Logs(
			streamCtx, ct.ID,
			new(containers.LogOptions).
				WithFollow(true).
				WithStdout(true).
				WithStderr(true).
				WithTail("all"),
			stdout, stderr,
		)
		close(logErr)
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); emitLines(stdout, lines) }()
	go func() { defer wg.Done(); emitLines(stderr, lines) }()
	go func() { wg.Wait(); close(lines) }()

	return &LogStream{Lines: lines, Err: logErr, cancel: cancel}
}

// Close stops following the logs.
func (ls *LogStream) Close() {
	if ls.cancel != nil {
		ls.cancel()
	}
}

// emitLines splits arbitrary chunks into complete lines.
func emitLines(in <-chan string, out chan<- string) {
	var buf strings.Builder
	for chunk := range in {
		buf.WriteString(chunk)
		for {
			s := buf.String()
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				break
			}
			out <- strings.TrimRight(s[:i], "\r")
			buf.Reset()
			buf.WriteString(s[i+1:])
		}
	}
	if rem := buf.String(); rem != "" {
		out <- strings.TrimRight(rem, "\r")
	}
}

// Wait blocks until the container exits or the timeout elapses.
func (ct *Container) Wait(timeout time.Duration) (int32, error) {
	reqCtx, cancel := context.WithTimeout(ct.conn, timeout)
	defer cancel()
	code, err := containers.Wait(reqCtx, ct.ID, nil)
	if err != nil {
		return code, fmt.Errorf("wait container: %w", err)
	}
	return code, nil
}

// Kill force-stops the container.
func (ct *Container) Kill(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ct.conn, 15*time.Second)
	defer cancel()
	if err := containers.Kill(reqCtx, ct.ID, new(containers.KillOptions).WithSignal("SIGKILL")); err != nil {
		return fmt.Errorf("kill container: %w", err)
	}
	return nil
}

// Remove deletes the container (and its anonymous volumes).
func (ct *Container) Remove(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ct.conn, 30*time.Second)
	defer cancel()
	if _, err := containers.Remove(reqCtx, ct.ID, new(containers.RemoveOptions).WithForce(true).WithVolumes(true)); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}
