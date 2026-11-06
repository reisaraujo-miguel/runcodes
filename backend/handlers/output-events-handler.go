package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/config"
	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"

	"golang.org/x/net/websocket"
)

// outputSocketWriteTimeout bounds one frame write. A client that stops reading
// must not pin the handler (and the judge subscription) forever.
const outputSocketWriteTimeout = 10 * time.Second

/*
OutputFilesSnapshot is what the output-files WebSocket sends: every generated
file the owner may download, the per-case upload states, and whether the run has
ended. The last frame always has complete=true, after which the server closes.
*/
type OutputFilesSnapshot struct {
	Type      string                      `json:"type"`
	Files     []OutputFileEntry           `json:"files"`
	Artifacts []models.SubmissionArtifact `json:"artifacts"`
	Complete  bool                        `json:"complete"`
}

/*
StreamSubmissionOutputEvents upgrades GET
/api/v1/submissions/{id}/output-events to a WebSocket that pushes the generated
files and upload states. The results panel no longer polls the listing: each
judge artifact event produces one frame, and the final frame is sent when the
judge stream ends.
*/
func StreamSubmissionOutputEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	commitID, ok := ownedCommitID(w, r)
	if !ok {
		return
	}

	snapshot, err := services.GetCommitSnapshot(ctx, commitID)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	terminal := judge.IsTerminalStatus(snapshot.Commit.Status)

	server := websocket.Server{
		Handshake: outputSocketOriginAllowed,
		Handler: func(conn *websocket.Conn) {
			defer conn.Close()

			if terminal {
				initial, err := outputFilesSnapshot(ctx, commitID, snapshot.Artifacts)
				if err != nil {
					slog.WarnContext(ctx, "could not build the final generated-file listing",
						slog.Int64("commit_id", commitID),
						slog.String("error", err.Error()),
					)
					return
				}
				initial.Complete = true
				_ = sendOutputSnapshot(conn, initial)
				return
			}

			// Subscribe before reading the state: an artifact event that lands
			// between the read and the subscription would otherwise be lost
			// until the next upload (or the final frame).
			subscription, err := judge.Subscribe(commitID)
			if err != nil {
				// The run settled between the snapshot and the subscription.
				// Send the final state instead of closing a socket the client
				// would have to reopen.
				if errors.Is(err, judge.ErrStreamClosed) {
					sendFinalOutputSnapshot(ctx, conn, commitID)
				}
				return
			}
			defer subscription.Close()

			initialArtifacts, err := services.ListCommitArtifacts(ctx, commitID)
			if err != nil {
				slog.WarnContext(ctx, "could not read the generated-file state",
					slog.Int64("commit_id", commitID),
					slog.String("error", err.Error()),
				)
				initialArtifacts = snapshot.Artifacts
			}
			initial, err := outputFilesSnapshot(ctx, commitID, initialArtifacts)
			if err != nil {
				slog.WarnContext(ctx, "could not build the generated-file listing",
					slog.Int64("commit_id", commitID),
					slog.String("error", err.Error()),
				)
				return
			}
			if err := sendOutputSnapshot(conn, initial); err != nil {
				return
			}

			// The client never sends application frames. Reading here is what
			// notices a disconnect and lets the library answer control frames.
			readErr := make(chan struct{}, 1)
			go func() {
				for {
					var message string
					if err := websocket.Message.Receive(conn, &message); err != nil {
						readErr <- struct{}{}
						return
					}
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case <-readErr:
					return
				case frame, ok := <-subscription.Events:
					if !ok {
						sendFinalOutputSnapshot(ctx, conn, commitID)
						return
					}
					if sseFrameEvent(frame) != "artifact" {
						continue
					}

					artifacts, err := services.ListCommitArtifacts(ctx, commitID)
					if err != nil {
						slog.WarnContext(ctx, "could not refresh generated-file state",
							slog.Int64("commit_id", commitID),
							slog.String("error", err.Error()),
						)
						continue
					}
					update, err := outputFilesSnapshot(ctx, commitID, artifacts)
					if err != nil {
						slog.WarnContext(ctx, "could not refresh generated-file listing",
							slog.Int64("commit_id", commitID),
							slog.String("error", err.Error()),
						)
						continue
					}
					if err := sendOutputSnapshot(conn, update); err != nil {
						return
					}
				}
			}
		},
	}

	server.ServeHTTP(w, r)
}

/*
outputSocketOriginAllowed accepts the site's own origin and the configured SPA
origin (the Vite dev server while it calls the API directly). The WebSocket
library only checks that Origin parses; without this comparison another site
could open the authenticated socket.
*/
func outputSocketOriginAllowed(wsConfig *websocket.Config, r *http.Request) error {
	origin, err := websocket.Origin(wsConfig, r)
	if err != nil {
		return err
	}
	if origin == nil {
		return errors.New("websocket request has no Origin")
	}

	if strings.EqualFold(origin.Host, r.Host) {
		return nil
	}

	configured, err := url.Parse(config.Get().FrontendOrigin)
	if err != nil {
		return fmt.Errorf("invalid FRONTEND_ORIGIN: %w", err)
	}
	if strings.EqualFold(origin.Host, configured.Host) {
		return nil
	}

	return fmt.Errorf("websocket origin %q is not allowed", origin.String())
}

/*
outputFilesSnapshot builds one frame from storage and the persisted upload
states. The run-level entry is only added when files exist: a commit whose every
case failed has statuses but nothing to download.
*/
func outputFilesSnapshot(
	ctx context.Context, commitID int64, artifacts []models.SubmissionArtifact,
) (OutputFilesSnapshot, error) {
	files, err := visibleOutputFiles(ctx, commitID)
	if err != nil {
		return OutputFilesSnapshot{}, err
	}

	entries := make([]OutputFileEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, OutputFileEntry{Path: file.Path, Size: file.Size})
	}

	if len(entries) > 0 && !hasRunLevelArtifact(artifacts) {
		artifacts = append(artifacts, models.SubmissionArtifact{Kind: "output"})
	}

	return OutputFilesSnapshot{
		Type:      "outputfiles",
		Files:     entries,
		Artifacts: artifacts,
	}, nil
}

func hasRunLevelArtifact(artifacts []models.SubmissionArtifact) bool {
	for _, artifact := range artifacts {
		if artifact.TestCaseID == nil {
			return true
		}
	}
	return false
}

func sendFinalOutputSnapshot(
	ctx context.Context, conn *websocket.Conn, commitID int64,
) {
	artifacts, err := services.ListCommitArtifacts(ctx, commitID)
	if err != nil {
		slog.WarnContext(ctx, "could not read the final generated-file state",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return
	}

	// The judge stream ended, so a pending upload will never report again.
	services.SettlePendingArtifacts("completed", artifacts)

	final, err := outputFilesSnapshot(ctx, commitID, artifacts)
	if err != nil {
		slog.WarnContext(ctx, "could not build the final generated-file listing",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
		return
	}
	final.Complete = true
	_ = sendOutputSnapshot(conn, final)
}

func sendOutputSnapshot(conn *websocket.Conn, snapshot OutputFilesSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	_ = conn.SetWriteDeadline(time.Now().Add(outputSocketWriteTimeout))
	return websocket.Message.Send(conn, string(data))
}

// sseFrameEvent reads the event name from a formatted SSE frame without parsing
// its (possibly large) JSON payload.
func sseFrameEvent(frame []byte) string {
	const prefix = "event: "
	if !bytes.HasPrefix(frame, []byte(prefix)) {
		return ""
	}

	end := bytes.IndexByte(frame, '\n')
	if end < 0 {
		return ""
	}

	return strings.TrimSpace(string(frame[len(prefix):end]))
}
