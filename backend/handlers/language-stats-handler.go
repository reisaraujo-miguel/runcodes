package handlers

import (
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
GetLanguageStatistics returns the platform-wide language report. The route is
behind the admin role; the report aggregates across every class, so there is no
per-class authorization to apply here.
*/
func GetLanguageStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	statistics, err := services.GetLanguageStatistics(ctx)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, statistics)
}

/*
GetOfferingLanguageStatistics returns the same report scoped to one class. The
service checks that the caller owns or teaches the class; students and monitors
receive the same not-found answer as any other author-only view.
*/
func GetOfferingLanguageStatistics(w http.ResponseWriter, r *http.Request) {
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

	statistics, err := services.GetOfferingLanguageStatistics(
		ctx, offeringID, claims,
	)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, statistics)
}
