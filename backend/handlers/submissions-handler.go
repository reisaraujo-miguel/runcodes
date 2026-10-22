package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/runcodes-icmc/runcodes/judge"
	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

// ssePingInterval is how often an idle event stream sends a comment heartbeat.
const ssePingInterval = 15 * time.Second

// sseWriteTimeout bounds a single write to an SSE client. The server has no
// global WriteTimeout because the stream is long-lived, so each frame refreshes
// its own deadline: a client that stops reading is disconnected instead of
// pinning the handler and its hub subscription forever.
const sseWriteTimeout = 30 * time.Second

/*
writeSSE writes one frame and flushes it, refreshing the connection's write
deadline. Response wrappers that do not expose deadlines are still served; they
simply keep the previous behavior.
*/
func writeSSE(
	w http.ResponseWriter,
	controller *http.ResponseController,
	flusher http.Flusher,
	payload []byte,
) error {
	_ = controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout))

	if _, err := w.Write(payload); err != nil {
		return err
	}
	flusher.Flush()

	return nil
}

/*
CreateSubmission handles a new source code submission. The body must be a
multipart/form-data with an "exercise_id" field and a "file" field.
*/
func CreateSubmission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error retrieving claims from context",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	userIDRaw, ok := claims["id"].(float64)
	if !ok {
		slog.ErrorContext(ctx, "invalid user id claim type",
			slog.Any("claim_id", claims["id"]),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Cap the whole body, including multipart overhead.
	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxSubmissionBytes)+(1<<20),
	)

	if err := r.ParseMultipartForm(services.MaxSubmissionBytes); err != nil {
		msg := "invalid submission payload"
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			msg = "submission file is too large"
			slog.InfoContext(ctx, "submission exceeded the size limit",
				slog.Any("user_id", claims["id"]),
			)
			WriteResponse(w, http.StatusRequestEntityTooLarge,
				models.Error{Message: msg},
			)
			return
		}
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	exerciseID, err := strconv.ParseInt(r.FormValue("exercise_id"), 10, 64)
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise_id"},
		)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "file is required"},
		)
		return
	}
	defer file.Close()

	if header.Size > services.MaxSubmissionBytes {
		slog.InfoContext(ctx, "submission file is too large",
			slog.Any("user_id", claims["id"]),
			slog.Int64("size", header.Size),
		)
		WriteResponse(w, http.StatusRequestEntityTooLarge,
			models.Error{Message: "submission file is too large"},
		)
		return
	}

	input := services.SubmissionInput{
		UserID:     int64(userIDRaw),
		ExerciseID: exerciseID,
		IP: services.ParseClientIP(
			middleware.GetClientIP(r.Context()), r.RemoteAddr,
		),
		Filename:    header.Filename,
		File:        file,
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
	}

	commitID, err := services.CreateSubmission(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExerciseNotFound):
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: services.ErrExerciseNotFound.Error()},
			)
		case errors.Is(err, services.ErrNotEnrolled):
			WriteResponse(w, http.StatusForbidden,
				models.Error{Message: services.ErrNotEnrolled.Error()},
			)
		case errors.Is(err, services.ErrDeadlinePassed):
			WriteResponse(w, http.StatusUnprocessableEntity,
				models.Error{Message: services.ErrDeadlinePassed.Error()},
			)
		case errors.Is(err, services.ErrInvalidFileType):
			WriteResponse(w, http.StatusBadRequest,
				models.Error{Message: services.ErrInvalidFileType.Error()},
			)
		case errors.Is(err, services.ErrSubmissionLimit):
			w.Header().Set("Retry-After", "30")
			WriteResponse(w, http.StatusTooManyRequests,
				models.Error{Message: services.ErrSubmissionLimit.Error()},
			)
		case errors.Is(err, services.ErrQueueFull):
			w.Header().Set("Retry-After", "30")
			WriteResponse(w, http.StatusServiceUnavailable,
				models.Error{Message: services.ErrQueueFull.Error()},
			)
		case errors.Is(err, services.ErrJudgeUnavailable):
			WriteResponse(w, http.StatusServiceUnavailable,
				models.Error{Message: services.ErrJudgeUnavailable.Error()},
			)
		default:
			slog.ErrorContext(ctx, "failed to create submission",
				slog.String("error", err.Error()),
				slog.Any("user_id", claims["id"]),
			)
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
		}
		return
	}

	WriteResponse(w, http.StatusCreated, models.CreateSubmissionResponse{
		CommitID:  commitID,
		Status:    "queued",
		EventsURL: "/api/v1/submissions/" + strconv.FormatInt(commitID, 10) + "/events",
	})
}

/*
ListSubmissionHistory handles GET /api/v1/exercises/{id}/submissions. It returns
only the caller's own submissions, newest first.
*/
func ListSubmissionHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}
	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	submissions, err := services.ListMyExerciseSubmissions(
		ctx, exerciseID, int64(userID),
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, submissions)
}

/*
GetSubmissionSource handles GET /api/v1/submissions/{id}/source. It streams the
original submitted file so the client can either download it or restore its
contents into the editor. The source belongs to the caller alone.
*/
func GetSubmissionSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}
	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	commitID, err := pathID(r, "id")
	if err != nil || commitID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid commit id"},
		)
		return
	}

	source, err := services.GetMySubmissionSource(ctx, commitID, int64(userID))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCommitNotFound):
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: services.ErrCommitNotFound.Error()},
			)
		case errors.Is(err, services.ErrCommitSourceNotFound):
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: services.ErrCommitSourceNotFound.Error()},
			)
		default:
			writeServiceError(ctx, w, err)
		}
		return
	}
	defer source.Body.Close()

	extension := strings.ToLower(filepath.Ext(source.Filename))
	contentType := source.ContentType
	if contentType == "" || contentType == "application/octet-stream" {
		if inferred := mime.TypeByExtension(extension); inferred != "" {
			contentType = inferred
		}
	}
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q", source.Filename),
	)
	w.Header().Set("Cache-Control", "private, no-store")
	if source.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(source.Size, 10))
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(newDeadlineWriter(w), source.Body); err != nil {
		slog.ErrorContext(ctx, "failed to stream submitted source",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
	}
}

/*
GetSubmissionOutput handles GET /api/v1/submissions/{id}/output. It proxies the
generated output archive from the judge so the browser can download it through
the authenticated API. The archive is available to the commit's owner only.
*/
func GetSubmissionOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}
	userID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	commitID, err := pathID(r, "id")
	if err != nil || commitID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid commit id"},
		)
		return
	}

	if err := services.VerifyMyCommit(ctx, commitID, int64(userID)); err != nil {
		if errors.Is(err, services.ErrCommitNotFound) {
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: services.ErrCommitNotFound.Error()},
			)
			return
		}
		writeServiceError(ctx, w, err)
		return
	}

	archive, err := judge.OpenOutput(ctx, commitID)
	if err != nil {
		switch {
		case errors.Is(err, judge.ErrOutputNotFound):
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: "no output was generated for this submission"},
			)
		default:
			slog.ErrorContext(ctx, "failed to fetch generated output",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
			WriteResponse(w, http.StatusBadGateway,
				models.Error{Message: "could not fetch the generated output"},
			)
		}
		return
	}
	defer archive.Body.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%q",
			strconv.FormatInt(commitID, 10)+".zip"),
	)
	w.Header().Set("Cache-Control", "private, no-store")
	if archive.ContentLength > 0 {
		w.Header().Set("Content-Length",
			strconv.FormatInt(archive.ContentLength, 10),
		)
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(newDeadlineWriter(w), archive.Body); err != nil {
		slog.ErrorContext(ctx, "failed to stream generated output",
			slog.Int64("commit_id", commitID),
			slog.String("error", err.Error()),
		)
	}
}

/*
StreamSubmissionEvents streams a commit's judging events as Server-Sent Events.
It starts with a snapshot of the persisted state, then relays live judge events
fanned out through the per-commit hub.
*/
func StreamSubmissionEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error retrieving claims from context",
			slog.String("error", err.Error()),
		)
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	userID, okID := claims["id"].(float64)
	role, _ := claims["role"].(string)
	if !okID {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	commitID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || commitID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid commit id"},
		)
		return
	}

	snapshot, err := services.GetCommitSnapshot(ctx, commitID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCommitNotFound):
			WriteResponse(w, http.StatusNotFound,
				models.Error{Message: services.ErrCommitNotFound.Error()},
			)
		default:
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
		}
		return
	}

	// A commit with a NULL user_id (the schema allows it and the judge
	// integration creates such rows) is not owned by the requester. Only the
	// owner or a member of staff who teaches the commit's offering may subscribe;
	// being a professor elsewhere on the platform is not enough.
	owned := snapshot.Commit.UserID != nil && *snapshot.Commit.UserID == int64(userID)
	isStaff := false

	if !owned {
		allowed := false
		if snapshot.Commit.ExerciseID != nil {
			var authErr error
			allowed, authErr = services.CanAuthorExercise(ctx, *snapshot.Commit.ExerciseID, claims)
			if authErr != nil {
				if !errors.Is(authErr, services.ErrExerciseNotFound) {
					slog.ErrorContext(ctx, "error authorizing commit viewer",
						slog.Int64("commit_id", commitID),
						slog.String("error", authErr.Error()),
					)
					WriteResponse(w, http.StatusInternalServerError,
						models.Error{Message: services.ErrServer.Error()},
					)
					return
				}
				allowed = false
			}
		}
		if !allowed {
			WriteResponse(w, http.StatusForbidden,
				models.Error{Message: services.ErrCommitForbidden.Error()},
			)
			return
		}
		isStaff = true
	} else if role == "professor" || role == "admin" {
		// The owner may also be staff of this class (a professor trying their own
		// exercise). They keep the full result; a student owner does not.
		if snapshot.Commit.ExerciseID != nil {
			staff, err := services.CanAuthorExercise(
				ctx, *snapshot.Commit.ExerciseID, claims,
			)
			if err == nil {
				isStaff = staff
			}
		}
	}

	// These fields are implementation details of the backend/judge handshake.
	// No viewer needs them, and they name the submitter and an internal object.
	snapshot.Commit.S3Key = nil
	snapshot.Commit.IP = nil

	// A case whose output the author did not release is redacted for students,
	// in the snapshot and in every live frame. The judge sends the raw output to
	// the backend, so this is the enforcement point.
	var hidden map[int64]struct{}
	if !isStaff {
		hidden, err = services.HiddenTestCaseIDs(ctx, commitID)
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError,
				models.Error{Message: services.ErrServer.Error()},
			)
			return
		}
		services.RedactSnapshotOutput(snapshot, hidden)
	}

	// Artifact events are transient and never persisted. Ask the judge whether
	// the generated archive exists so a reloading client can show the files
	// section. The archive can appear before the finished event, so check even
	// while the run is still in progress. Only the owner can download it, so do
	// not advertise it on a privileged viewer's snapshot.
	if owned {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		exists, err := judge.OutputExists(checkCtx, commitID)
		cancel()
		if err != nil {
			slog.WarnContext(ctx, "could not check generated output",
				slog.Int64("commit_id", commitID),
				slog.String("error", err.Error()),
			)
		} else if exists {
			snapshot.Artifacts = append(snapshot.Artifacts,
				models.SubmissionArtifact{Kind: "output"},
			)
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.ErrorContext(ctx, "response writer does not support flushing")
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}
	controller := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		slog.ErrorContext(ctx, "failed to encode snapshot",
			slog.String("error", err.Error()),
		)
		return
	}
	if err := writeSSE(w, controller, flusher,
		judge.FormatSSEFrame("snapshot", 0, snapshotJSON)); err != nil {
		return
	}

	if judge.IsTerminalStatus(snapshot.Commit.Status) {
		return
	}

	subscription, err := judge.Subscribe(commitID)
	if err != nil {
		return
	}
	defer subscription.Close()

	ticker := time.NewTicker(ssePingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-subscription.Events:
			if !ok {
				return
			}
			if !isStaff {
				frame = redactCaseResultFrame(frame, hidden)
				if frame == nil {
					// A hidden case_result frame that could not be parsed: dropping
					// it is safer than forwarding output the author hid.
					slog.ErrorContext(ctx, "dropping malformed case_result frame",
						slog.Int64("commit_id", commitID),
					)
					continue
				}
			}
			if err := writeSSE(w, controller, flusher, frame); err != nil {
				return
			}
		case <-ticker.C:
			if err := writeSSE(w, controller, flusher, []byte(": ping\n\n")); err != nil {
				return
			}
		}
	}
}
