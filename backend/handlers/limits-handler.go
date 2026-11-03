package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
GetPlatformLimits returns the platform-wide default limits, the last layer of the
resolution chain. The frontend shows them as the inherited value and as the
placeholder of every empty limit field.
*/
func GetPlatformLimits(w http.ResponseWriter, r *http.Request) {
	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	WriteResponse(w, http.StatusOK, services.PlatformLimits())
}

/*
GetExerciseLanguageLimits returns one entry per language the exercise allows,
with the exercise's override and the values and sources that apply when a test
case has no override of its own.
*/
func GetExerciseLanguageLimits(w http.ResponseWriter, r *http.Request) {
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

	limits, err := services.ListExerciseLanguageLimits(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, limits)
}

/*
ReplaceExerciseLanguageLimits stores the exercise's per-language defaults. The
body is the whole set: a language left out, or with every field null, inherits
the platform default.
*/
func ReplaceExerciseLanguageLimits(w http.ResponseWriter, r *http.Request) {
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

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "could not read the request body"},
		)
		return
	}

	var limits []models.LanguageLimits
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&limits); err != nil {
		WriteResponse(w, http.StatusUnprocessableEntity,
			models.Error{Message: services.ErrInvalidLimits.Error()},
		)
		return
	}

	updated, err := services.ReplaceExerciseLanguageLimits(
		ctx, exerciseID, limits, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, updated)
}
