package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/contest_service"
)

func (a *Api) HandlerSetUsersInContest(w http.ResponseWriter, r *http.Request) {
	// get the users from body
	type params struct {
		ContestID uuid.UUID `json:"contest_id"`
		UserNames []string  `json:"user_names"`
	}
	var request params
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// set users
	err = a.ContestServiceConfig.RegisterUsersToContest(
		r.Context(),
		request.ContestID,
		request.UserNames,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("users set successfully"))
}

func (a *Api) HandlerSetProblemsInContest(w http.ResponseWriter, r *http.Request) {
	// get the users from body
	type params struct {
		ContestID uuid.UUID                        `json:"contest_id"`
		Problems  []contest_service.ContestProblem `json:"problems"`
	}
	var request params
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// set problems
	err = a.ContestServiceConfig.SetProblemsInContest(
		r.Context(),
		request.ContestID,
		request.Problems,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("problems set successfully"))
}

func (a *Api) HandlerUpdateContest(w http.ResponseWriter, r *http.Request) {
	// parse the request
	var contest contest_service.Contest
	err := decodeJsonBody(r.Body, &contest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// update the contest using service
	serviceContest, err := a.ContestServiceConfig.UpdateContest(r.Context(), contest)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(serviceContest)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", serviceContest, err.Error())
		http.Error(w, "contest updated but cannot prepare response", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
