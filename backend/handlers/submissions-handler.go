package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// ssePingInterval is how often an idle event stream sends a comment heartbeat.
const ssePingInterval = 15 * time.Second

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
			r.Header.Get("X-Forwarded-For"), r.RemoteAddr,
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

	owner := int64(userID)
	if snapshot.Commit.UserID != nil {
		owner = *snapshot.Commit.UserID
	}
	if int64(userID) != owner && role != "professor" && role != "admin" {
		WriteResponse(w, http.StatusForbidden,
			models.Error{Message: services.ErrCommitForbidden.Error()},
		)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.ErrorContext(ctx, "response writer does not support flushing")
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: services.ErrServer.Error()},
		)
		return
	}

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
	if _, err := w.Write(services.FormatSSEFrame("snapshot", 0, snapshotJSON)); err != nil {
		return
	}
	flusher.Flush()

	if services.IsTerminalStatus(snapshot.Commit.Status) {
		return
	}

	subscription, err := services.SubscribeCommit(commitID)
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
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
