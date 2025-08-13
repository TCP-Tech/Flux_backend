package api

import (
	"net/http"

	"github.com/google/uuid"
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
	err = a.ContestService.RegisterUsersToContest(
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
	err = a.ContestService.SetProblemsInContest(
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
