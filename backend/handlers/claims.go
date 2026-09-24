package handlers

import (
	"log/slog"
)

/*
claimsID reads the numeric user id of the session claims. A claim that is missing
or of the wrong type is an internal error in the token, never an authorization
pass: the callers answer it with 401.
*/
func claimsID(claims map[string]any) (int, bool) {
	id, ok := claims["id"].(float64)
	if !ok {
		slog.Error("invalid user id claim", slog.Any("claim_id", claims["id"]))
		return 0, false
	}
	return int(id), true
}
