package handlers

import (
	"log/slog"
	"net/http"
	"os"

	"runcodes/utils"
)

var debugModeEnv string = "DEBUG_MODE"

func GenerateDebugToken(w http.ResponseWriter, r *http.Request) {
	if os.Getenv(debugModeEnv) != "true" {
		utils.WriteResponse(w, http.StatusNotFound, false, "not found", nil)
		return
	}

	_, token, err := utils.TokenAuth.Encode(map[string]any{"user": 123})
	if err != nil {
		slog.Error(err.Error())
		utils.WriteResponse(w, http.StatusInternalServerError, false, "an error ocurred", err)
		return
	}

	utils.WriteResponse(w, http.StatusAccepted, true, "Your debug token is", map[string]string{"token": token})
}
