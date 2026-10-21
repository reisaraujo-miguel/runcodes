/*
Package judge is the backend's side of the judge integration: the HTTP client
for the judge's API, the SSE event stream (parsing, fan-out and persistence) and
the sweeper that settles commits the judge never finished.
*/
package judge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
)

var (
	// ErrOutputNotFound reports that the judge has no generated output archive
	// for a commit.
	ErrOutputNotFound = errors.New("output archive not found")

	// readyClient bounds the pre-registration readiness probe.
	readyClient = &http.Client{Timeout: 2 * time.Second}

	// streamClient has no overall timeout: SSE responses are long-lived.
	streamClient = &http.Client{}

	// outputClient bounds a download of a generated archive. The judge is on the
	// internal network; a request that stalls must still release the handler.
	outputClient = &http.Client{Timeout: 60 * time.Second}
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
OpenOutput opens a commit's generated output archive on the judge. The caller
must close the body. A missing archive is reported as ErrOutputNotFound.
*/
func OpenOutput(ctx context.Context, commitID int64) (*http.Response, error) {
	req, err := newRequest(
		ctx, http.MethodGet,
		"/v1/runs/"+strconv.FormatInt(commitID, 10)+"/output", nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zip")

	resp, err := outputClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opening judge output: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrOutputNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("judge output returned status %d", resp.StatusCode)
	}

	return resp, nil
}

/*
OutputExists reports whether the judge has a generated output archive for a
commit. The snapshot uses it to restore the "generated files" section after a
page reload, when the transient artifact event is no longer in the stream.
*/
func OutputExists(ctx context.Context, commitID int64) (bool, error) {
	req, err := newRequest(
		ctx, http.MethodHead,
		"/v1/runs/"+strconv.FormatInt(commitID, 10)+"/output", nil,
	)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/zip")

	resp, err := outputClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("checking judge output: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("judge output check returned status %d", resp.StatusCode)
	}
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
