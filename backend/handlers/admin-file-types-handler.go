package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
AdminListFileTypes lists the whole allowed-file-type catalog for the admin
panel, including entries that are hidden from the professor-facing picker.
*/
func AdminListFileTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	types, err := services.AdminListAllowedFileTypes(ctx)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, types)
}

/*
AdminCreateFileType registers a new file type without a redeploy. The catalog
cache is dropped by the service, so the professor picker sees it immediately.
*/
func AdminCreateFileType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	var req models.CreateAllowedFileTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file type request"},
		)
		return
	}

	fileType, err := services.CreateAllowedFileType(ctx, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusCreated, fileType)
}

/*
AdminUpdateFileType edits a catalog entry, including hiding it with
is_available=false.
*/
func AdminUpdateFileType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	id, err := pathID(r, "id")
	if err != nil || id <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file type id"},
		)
		return
	}

	var req models.UpdateAllowedFileTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file type request"},
		)
		return
	}

	fileType, err := services.UpdateAllowedFileType(ctx, id, &req)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, fileType)
}

/*
AdminDeleteFileType removes an unused catalog entry. A referenced entry is
refused with 409 so the admin can disable it instead.
*/
func AdminDeleteFileType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	id, err := pathID(r, "id")
	if err != nil || id <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid file type id"},
		)
		return
	}

	if err := services.DeleteAllowedFileType(ctx, id); err != nil {
		if errors.Is(err, services.ErrFileTypeInUse) {
			slog.InfoContext(ctx, "refused to delete a referenced file type",
				slog.Int64("file_type_id", id),
			)
		}
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
