package handlers

import "net/http"

/*
Health is the liveness probe used by container orchestrators. It deliberately does not touch the database: a temporary
database outage must not make the process look dead and trigger restarts.
*/
func Health(w http.ResponseWriter, _ *http.Request) {
	WriteResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
