package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/runcodes-icmc/runcodes/alerts"
	"github.com/runcodes-icmc/runcodes/models"
)

/*
AdminListAlerts lists the platform's operational alerts, newest first
(`?limit=&offset=&open=1`). The response also carries the number of alerts still
open, which the panel shows without a second request.
*/
func AdminListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := tokenClaims(w, r); !ok {
		return
	}

	page, err := alerts.List(ctx, alerts.ListOptions{
		OnlyOpen: r.URL.Query().Get("open") == "1",
		Limit:    queryInt(r, "limit"),
		Offset:   queryInt(r, "offset"),
	})
	if err != nil {
		writeAlertError(w, err)
		return
	}

	WriteResponse(w, http.StatusOK, page)
}

/*
AdminAcknowledgeAlert marks one alert as seen by the calling administrator.

It is idempotent: acknowledging an alert that somebody already acknowledged
returns that alert with its original acknowledgement, so a double click (or a
retry after a lost response) is not an error. Nothing is acknowledged on the
administrators' behalf when it fails.
*/
func AdminAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := tokenClaims(w, r)
	if !ok {
		return
	}

	adminID, ok := claimsID(claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	alertID, err := pathID(r, "id")
	if err != nil || alertID <= 0 {
		WriteResponse(w, http.StatusBadRequest,
			models.Error{Message: "invalid alert id"},
		)
		return
	}

	alert, err := alerts.Acknowledge(ctx, alertID, adminID)
	if err != nil {
		writeAlertError(w, err)
		return
	}

	WriteResponse(w, http.StatusOK, alert)
}

/*
writeAlertError maps the alert package's sentinel errors to HTTP responses. The
storage failure is already logged where it happened; the caller only needs a
status.
*/
func writeAlertError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, alerts.ErrNotFound):
		WriteResponse(w, http.StatusNotFound,
			models.Error{Message: err.Error()},
		)
	case errors.Is(err, alerts.ErrServer):
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: err.Error()},
		)
	default:
		slog.Error("unexpected alert error", slog.String("error", err.Error()))
		WriteResponse(w, http.StatusInternalServerError,
			models.Error{Message: alerts.ErrServer.Error()},
		)
	}
}
