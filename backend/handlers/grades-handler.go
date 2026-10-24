package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
GetOfferingGradebook returns the grade table and the statistics of an offering.
Only the owner, a co-professor or the admin panel may read it: the table contains
every student's grades.
*/
func GetOfferingGradebook(w http.ResponseWriter, r *http.Request) {
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

	gradebook, err := services.GetOfferingGradebook(ctx, offeringID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, gradebook)
}

/*
ExportOfferingGradesCSV streams the same grade table as a CSV file. It is a
separate endpoint so the browser can download it directly instead of building a
blob from the JSON.
*/
func ExportOfferingGradesCSV(w http.ResponseWriter, r *http.Request) {
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

	gradebook, err := services.GetOfferingGradebook(ctx, offeringID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	filename := "notas-turma-" + strconv.FormatInt(offeringID, 10) + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+filename+`"`,
	)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// The UTF-8 byte-order mark makes Excel open accented names correctly.
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		slog.DebugContext(ctx, "client stopped the CSV download",
			slog.String("error", err.Error()),
		)
		return
	}

	if err := services.WriteOfferingGradebookCSV(
		newDeadlineWriter(w), gradebook,
	); err != nil {
		slog.ErrorContext(ctx, "error writing the grades CSV",
			slog.Int64("offering_id", offeringID),
			slog.String("error", err.Error()),
		)
	}
}
