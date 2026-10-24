package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
	"github.com/runcodes-icmc/runcodes/validation"
)

/*
ExportOffering handles GET /api/v1/offerings/{id}/export: a zip with the class
metadata, exercises, test cases, compilation files and materials.
*/
func ExportOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	filename, archive, err := services.ExportOfferingArchive(
		ctx, offeringID, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+filename+`"`,
	)
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := newDeadlineWriter(w).Write(archive); err != nil {
		slog.DebugContext(ctx, "client stopped the offering download",
			slog.String("error", err.Error()),
		)
	}
}

/*
ImportOffering handles POST /api/v1/offerings/import. The multipart body carries
the archive and optional overrides for the new class name, description and end
date. The caller becomes the owner of the imported class.
*/
func ImportOffering(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxOfferingArchiveBytes)+(1<<20),
	)
	if err := r.ParseMultipartForm(services.MaxOfferingArchiveBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteResponse(w, http.StatusRequestEntityTooLarge,
				models.Error{Message: services.ErrArchiveTooLarge.Error()},
			)
			return
		}
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering archive payload"},
		)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	headers, ok := r.MultipartForm.File["archive"]
	if !ok || len(headers) == 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "archive is required"},
		)
		return
	}
	file, err := headers[0].Open()
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "could not read the archive"},
		)
		return
	}
	defer file.Close()

	archive, err := io.ReadAll(
		io.LimitReader(file, services.MaxOfferingArchiveBytes+1),
	)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "could not read the archive"},
		)
		return
	}
	if len(archive) > services.MaxOfferingArchiveBytes {
		WriteResponse(w, http.StatusRequestEntityTooLarge,
			models.Error{Message: services.ErrArchiveTooLarge.Error()},
		)
		return
	}

	options := services.OfferingImportOptions{
		Name: strings.TrimSpace(formValue(r.MultipartForm.Value, "name")),
	}
	if values, ok := r.MultipartForm.Value["description"]; ok && len(values) > 0 {
		description := values[0]
		options.Description = &description
	}
	if raw := strings.TrimSpace(
		formValue(r.MultipartForm.Value, "end_date"),
	); raw != "" {
		parsed, err := validation.ValidateDate(ctx, raw)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest,
				models.Error{Message: "invalid end date"},
			)
			return
		}
		options.EndDate = parsed
	}

	offering, err := services.ImportOfferingArchive(
		ctx, archive, options, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, offering)
}

/*
ExportOfferingSubmissions handles GET
/api/v1/offerings/{id}/submissions/export: the newest submission of every student
on every exercise, plus a CSV with the grades of those submissions.
*/
func ExportOfferingSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	if err := services.AuthorizeOfferingAuthor(ctx, offeringID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	filename := "turma-" + strconv.FormatInt(offeringID, 10) +
		"-ultimas-submissoes.zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+filename+`"`,
	)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, err := services.WriteOfferingSubmissionsArchive(
		ctx, offeringID, claims, newDeadlineWriter(w),
	); err != nil {
		// The headers are already committed; the connection is aborted so the
		// browser reports a failed download instead of saving a truncated zip.
		slog.ErrorContext(ctx, "error streaming the submissions archive",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		panic(http.ErrAbortHandler)
	}
}

/*
RecompileOfferingSubmissions handles POST
/api/v1/offerings/{id}/submissions/recompile. It queues a fresh run of every
student's newest submission on every exercise.
*/
func RecompileOfferingSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	result, err := services.RecompileOfferingSubmissions(
		ctx, offeringID, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusAccepted, result)
}

/*
NotifyOfferingMembers handles POST /api/v1/offerings/{id}/notify: an email to
every member of the class.
*/
func NotifyOfferingMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	offeringID, err := pathID(r, "id")
	if err != nil || offeringID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid offering id"},
		)
		return
	}

	var req models.OfferingEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid email request"},
		)
		return
	}

	result, err := services.SendOfferingEmail(ctx, offeringID, &req, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusAccepted, result)
}
