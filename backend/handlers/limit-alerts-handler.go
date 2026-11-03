package handlers

import (
	"net/http"

	"github.com/runcodes-icmc/runcodes/models"
	"github.com/runcodes-icmc/runcodes/services"
)

/*
ListExerciseLimitAlerts returns the unacknowledged limit alerts of an exercise:
the professor's notice that a language has been failing submissions because of a
limit.
*/
func ListExerciseLimitAlerts(w http.ResponseWriter, r *http.Request) {
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

	alerts, err := services.ListExerciseLimitAlerts(ctx, exerciseID, claims)
	if err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusOK, alerts)
}

/* AcknowledgeExerciseLimitAlert marks one limit alert as handled. */
func AcknowledgeExerciseLimitAlert(w http.ResponseWriter, r *http.Request) {
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
	alertID, err := pathID(r, "alertId")
	if err != nil || alertID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid alert id"},
		)
		return
	}

	if err := services.AcknowledgeExerciseLimitAlert(
		ctx, exerciseID, alertID, claims,
	); err != nil {
		writeServiceError(ctx, w, err)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
