package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

func parseSingleFileRequest(w http.ResponseWriter, r *http.Request) (*services.UploadedFile, bool) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(
		w, r.Body, int64(services.MaxAuthoringUploadBytes)+(1<<20),
	)

	if err := r.ParseMultipartForm(services.MaxAuthoringUploadBytes); err != nil {
		msg := "invalid file upload payload"
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteResponse(w, http.StatusRequestEntityTooLarge,
				models.Error{Message: "upload is too large"},
			)
			return nil, false
		}
		slog.ErrorContext(ctx, msg, slog.String("error", err.Error()))
		WriteResponse(w, http.StatusBadRequest, models.Error{Message: msg})
		return nil, false
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, err := firstUploadedFile(r.MultipartForm, "file")
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: err.Error()},
		)
		return nil, false
	}
	if file == nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "file is required"},
		)
		return nil, false
	}

	return file, true
}

/*
ListCompilationFiles handles GET /api/v1/exercises/{id}/compilation-files.
*/
func ListCompilationFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	files, err := services.ListCompilationFiles(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, files)
}

/*
CreateCompilationFile handles POST /api/v1/exercises/{id}/compilation-files.
*/
func CreateCompilationFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	file, ok := parseSingleFileRequest(w, r)
	if !ok {
		return
	}

	compilationFile, err := services.CreateCompilationFile(ctx, exerciseID, *file, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, compilationFile)
}

/*
GetCompilationFile handles GET
/api/v1/exercises/{id}/compilation-files/{fileId}.
*/
func GetCompilationFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	file, err := services.GetCompilationFile(ctx, exerciseID, fileID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, file)
}

/*
DeleteCompilationFile handles DELETE
/api/v1/exercises/{id}/compilation-files/{fileId}.
*/
func DeleteCompilationFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	if err := services.DeleteCompilationFile(ctx, exerciseID, fileID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

/*
ListAttachedFiles handles GET /api/v1/exercises/{id}/attached-files.
*/
func ListAttachedFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	files, err := services.ListAttachedFiles(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, files)
}

/*
GetAttachedFile handles GET /api/v1/exercises/{id}/attached-files/{fileId}.
It streams the stored attachment so students can read it in the browser; PDFs
are served inline so they can be embedded on the submission page.
*/
func GetAttachedFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	file, err := services.GetAttachedFile(ctx, exerciseID, fileID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}
	defer file.Body.Close()

	extension := strings.ToLower(filepath.Ext(file.Filename))
	contentType := file.ContentType
	if extension == ".pdf" {
		// The statement must render in the embedded viewer even when the upload
		// was stored with a generic content type.
		contentType = "application/pdf"
	} else if contentType == "" || contentType == "application/octet-stream" {
		if inferred := mime.TypeByExtension(extension); inferred != "" {
			contentType = inferred
		}
	}

	disposition := "attachment"
	if extension == ".pdf" || contentType == "application/pdf" {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("%s; filename=%q", disposition, file.Filename),
	)
	if file.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(newDeadlineWriter(w), file.Body); err != nil {
		slog.ErrorContext(ctx, "failed to stream attached file",
			slog.Int64("attached_file_id", fileID),
			slog.String("error", err.Error()),
		)
	}
}

/*
CreateAttachedFile handles POST /api/v1/exercises/{id}/attached-files.
*/
func CreateAttachedFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}

	file, ok := parseSingleFileRequest(w, r)
	if !ok {
		return
	}

	attached, err := services.CreateAttachedFile(ctx, exerciseID, *file, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, attached)
}

/*
DeleteAttachedFile handles DELETE
/api/v1/exercises/{id}/attached-files/{fileId}.
*/
func DeleteAttachedFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	exerciseID, err := pathID(r, "id")
	if err != nil || exerciseID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid exercise id"},
		)
		return
	}
	fileID, err := pathID(r, "fileId")
	if err != nil || fileID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file id"},
		)
		return
	}

	if err := services.DeleteAttachedFile(ctx, exerciseID, fileID, claims); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
