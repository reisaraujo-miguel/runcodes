package handlers

import (
	"log/slog"
	"net/http"

	"runcodes/utils"
)

func GenerateDebugToken(w http.ResponseWriter, r *http.Request) {
	_, token, err := utils.TokenAuth.Encode(map[string]any{"user": 123})
	if err != nil {
		slog.Error(err.Error())
		utils.WriteResponse(w, http.StatusInternalServerError, false, "an error ocurred", err)
		return
	}
	slog.Info("A debug token is", slog.String("token", token))

	utils.WriteResponse(w, http.StatusAccepted, true, "Your debug token is", map[string]string{"token": token})
}
