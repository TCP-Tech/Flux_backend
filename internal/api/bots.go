package api

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/submission_service"
)

func (a *Api) AddBot(w http.ResponseWriter, r *http.Request) {
	// decode bot from body
	var bot submission_service.Bot
	err := decodeJsonBody(r.Body, &bot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// add bot to db
	fluxBot, err := a.SubmissionService.AddBot(r.Context(), bot)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal bot and return
	responseBytes, err := json.Marshal(fluxBot)
	if err != nil {
		logrus.Errorf("cannot marshal %v: %v", fluxBot, err)
		http.Error(
			w,
			"bot added successfully but there was an error in preparing response",
			http.StatusInternalServerError,
		)
		return
	}

	// respond
	respondWithJson(w, http.StatusCreated, responseBytes)
}

func (a *Api) RefreshBots(w http.ResponseWriter, r *http.Request) {
	err := a.SubmissionService.RefreshBots(r.Context())
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("initiated bots refershment successfully"))
}