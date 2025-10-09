package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
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

func (a *Api) UpdateBot(w http.ResponseWriter, r *http.Request) {
	// decode bot from body
	var bot submission_service.Bot
	err := decodeJsonBody(r.Body, &bot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updatedBot, err := a.SubmissionService.UpdateBot(r.Context(), bot)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	bytes, err := json.Marshal(updatedBot)
	if err != nil {
		logrus.Error("cannot marshal %v: %v", updatedBot, err)
		respondWithJson(w, http.StatusInternalServerError, []byte("bot update but error in preparing response"))
		return
	}

	respondWithJson(w, http.StatusOK, bytes)
}

func (a *Api) DeleteBot(w http.ResponseWriter, r *http.Request) {
	// get its name from query
	botName := r.URL.Query().Get("name")

	// delete bot
	err := a.SubmissionService.DeleteBot(r.Context(), botName)
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("bot deleted successfully"))
}

func (a *Api) GetBots(w http.ResponseWriter, r *http.Request) {
	bots, err := a.SubmissionService.GetBots(r.Context())
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	bytes, err := json.Marshal(bots)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v: %w",
			flux_errors.ErrInternal,
			bots,
			err,
		)
		logrus.Error(err)
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, bytes)
}
