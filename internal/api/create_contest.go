package api

import (
	"encoding/json"
	"net/http"

	"github.com/tcp_snm/flux/internal/service/contest_service"
)

func (a *Api) HandlerCreateContest(w http.ResponseWriter, r *http.Request) {
	// parse the request
	var createContestRequest contest_service.CreateContestRequest
	err := decodeJsonBody(r.Body, &createContestRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create contest
	contest, err := a.ContestServiceConfig.CreateContest(
		r.Context(),
		createContestRequest,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marhsal
	response, err := json.Marshal(contest)
	if err != nil {
		http.Error(
			w, "contest created but error in preparing response",
			http.StatusInternalServerError,
		)
		return
	}

	// respond
	respondWithJson(w, http.StatusCreated, response)
}
