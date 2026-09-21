package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	judgeURLEnv     = "RUNCODES_JUDGE_URL"
	judgeTokenEnv   = "RUNCODES_JUDGE_TOKEN"
	defaultJudgeURL = "http://judge:9000"
)

var (
	// judgeReadyClient bounds the pre-registration readiness probe.
	judgeReadyClient = &http.Client{Timeout: 2 * time.Second}

	// judgeStreamClient has no overall timeout: SSE responses are long-lived.
	judgeStreamClient = &http.Client{}
)

/*
judgeBaseURL returns the judge base URL, without a trailing slash.
*/
func judgeBaseURL() string {
	url := os.Getenv(judgeURLEnv)
	if url == "" {
		url = defaultJudgeURL
	}
	return strings.TrimRight(url, "/")
}

/*
newJudgeRequest builds a request to the judge, adding the shared bearer token
when RUNCODES_JUDGE_TOKEN is set.
*/
func newJudgeRequest(
	ctx context.Context, method, path string, body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, judgeBaseURL()+path, body)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv(judgeTokenEnv); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

/*
JudgeReady checks the judge readiness endpoint (podman + postgres reachable).
It is used before registering a submission.
*/
func JudgeReady(ctx context.Context) error {
	req, err := newJudgeRequest(ctx, http.MethodGet, "/readyz", nil)
	if err != nil {
		return err
	}

	resp, err := judgeReadyClient.Do(req)
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
WakeJudge nudges the judge to process a commit. It is idempotent.
*/
func WakeJudge(ctx context.Context, commitID int64) error {
	req, err := newJudgeRequest(
		ctx, http.MethodPost, "/v1/runs/"+strconv.FormatInt(commitID, 10), nil,
	)
	if err != nil {
		return err
	}

	resp, err := judgeStreamClient.Do(req)
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
openJudgeEvents opens the judge SSE stream for a commit, resuming after "from"
when it is greater than zero.
*/
func openJudgeEvents(
	ctx context.Context, commitID int64, from int64,
) (*http.Response, error) {
	path := "/v1/runs/" + strconv.FormatInt(commitID, 10) + "/events"
	if from > 0 {
		path += "?from=" + strconv.FormatInt(from, 10)
	}

	req, err := newJudgeRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := judgeStreamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to judge events stream: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("judge events stream returned status %d", resp.StatusCode)
	}

	return resp, nil
}
