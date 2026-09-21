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

	// system.Version is a simple ping that returns the podman version.
	if _, err := system.Version(conn, nil); err != nil {
		return fmt.Errorf("podman version: %w", err)
	}

	return nil
}

// EnsureImage makes sure the image is present locally, pulling it if needed.
func (c *Client) EnsureImage(ctx context.Context, image string) error {
	// avoid pulling the same image multiple times in parallel
	c.mu.Lock()
	// check again after acquiring the lock, in case another goroutine pulled it
	already := c.ensured[image]
	c.mu.Unlock()

	if already {
		return nil
	}

	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}

	// check if the image exists locally
	exists, err := images.Exists(conn, image, nil)
	if err != nil {
		return fmt.Errorf("check image %s: %w", image, err)
	}

	// pull the image if it does not exist
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

	// force-remove the container and its anonymous volumes; ignore errors if it was already removed
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

	// generate the container spec
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
		// "z" relabels the workspace for container access (required on
		// SELinux hosts, ignored elsewhere); without it a rootless
		// container's root cannot write to the bind mount even at 0777.
		Options: []string{"rw", "z"},
	}}

	// create the container with the spec
	resp, err := containers.CreateWithSpec(conn, s, nil)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	return &Container{ID: resp.ID, cfg: rc, conn: conn}, nil
}

// Start starts the container.
func (ct *Container) Start(ctx context.Context) error {
	// use a 30-second timeout for starting the container; it should be fast
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
	// derive a cancellable context from the container's connection context
	streamCtx, cancel := context.WithCancel(ct.conn)

	stdout := make(chan string, 256)
	stderr := make(chan string, 256)
	lines := make(chan string, 256)
	logErr := make(chan error, 1)

	go func() {
		defer close(stdout)
		defer close(stderr)

		// follow the logs with a 5-minute timeout; it should be fast
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

	// merge stdout and stderr into a single line stream
	var wg sync.WaitGroup
	wg.Go(func() { emitLines(stdout, lines) })
	wg.Go(func() { emitLines(stderr, lines) })
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

		// split the buffer into lines and send them to the output channel
		for {
			s := buf.String()
			i := strings.IndexByte(s, '\n') // find the next newline

			if i < 0 {
				break // no complete line left in the buffer
			}

			out <- strings.TrimRight(s[:i], "\r") // send the line without trailing CR

			// remove the emitted line from the buffer and continue
			buf.Reset()
			buf.WriteString(s[i+1:])
		}
	}

	// send any remaining partial line in the buffer
	if rem := buf.String(); rem != "" {
		out <- strings.TrimRight(rem, "\r")
	}
}

// Wait blocks until the container exits or the timeout elapses.
func (ct *Container) Wait(timeout time.Duration) (int32, error) {
	reqCtx, cancel := context.WithTimeout(ct.conn, timeout)
	defer cancel()

	// wait for the container to exit and get its exit code
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

	// send SIGKILL to the container; this is a last resort if it does not exit cleanly
	if err := containers.Kill(reqCtx, ct.ID, new(containers.KillOptions).WithSignal("SIGKILL")); err != nil {
		return fmt.Errorf("kill container: %w", err)
	}

	return nil
}

// Remove deletes the container (and its anonymous volumes).
func (ct *Container) Remove(ctx context.Context) error {
	reqCtx, cancel := context.WithTimeout(ct.conn, 30*time.Second)
	defer cancel()

	// force-remove the container and its anonymous volumes; ignore errors if it was already removed
	if _, err := containers.Remove(reqCtx, ct.ID, new(containers.RemoveOptions).WithForce(true).WithVolumes(true)); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}

	return nil
}
