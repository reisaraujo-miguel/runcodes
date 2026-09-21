/*
Package judge is the backend's side of the judge integration: the HTTP client
for the judge's API, the SSE event stream (parsing, fan-out and persistence) and
the sweeper that settles commits the judge never finished.
*/
package judge

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

var (
	// readyClient bounds the pre-registration readiness probe.
	readyClient = &http.Client{Timeout: 2 * time.Second}

	// streamClient has no overall timeout: SSE responses are long-lived.
	streamClient = &http.Client{}
)

/*
newRequest builds a request to the judge, adding the shared bearer token when
one is configured.
*/
func newRequest(
	ctx context.Context, method, path string, body io.Reader,
) (*http.Request, error) {
	judgeConfig := config.Get().Judge

	req, err := http.NewRequestWithContext(ctx, method, judgeConfig.URL+path, body)
	if err != nil {
		return nil, err
	}

	if judgeConfig.Token != "" {
		req.Header.Set("Authorization", "Bearer "+judgeConfig.Token)
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

/*
Ready checks the judge readiness endpoint (podman + postgres reachable).
It is used before registering a submission.
*/
func Ready(ctx context.Context) error {
	req, err := newRequest(ctx, http.MethodGet, "/readyz", nil)
	if err != nil {
		return err
	}

	resp, err := readyClient.Do(req)
	if err != nil {
		return fmt.Errorf("judge readiness check failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("judge readiness check returned status %d", resp.StatusCode)
	}

	return nil
}

/*
Wake nudges the judge to process a commit. It is idempotent.
*/
func Wake(ctx context.Context, commitID int64) error {
	req, err := newRequest(
		ctx, http.MethodPost, "/v1/runs/"+strconv.FormatInt(commitID, 10), nil,
	)
	if err != nil {
		return err
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("waking judge: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("waking judge returned status %d", resp.StatusCode)
	}

	return nil
}

/*
openEvents opens the judge SSE stream for a commit, resuming after "from" when
it is greater than zero.
*/
func openEvents(
	ctx context.Context, commitID int64, from int64,
) (*http.Response, error) {
	path := "/v1/runs/" + strconv.FormatInt(commitID, 10) + "/events"
	if from > 0 {
		path += "?from=" + strconv.FormatInt(from, 10)
	}

	req, err := newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to judge events stream: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("judge events stream returned status %d", resp.StatusCode)
	}

	return resp, nil
}
