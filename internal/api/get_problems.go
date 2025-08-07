package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerGetProblemById(w http.ResponseWriter, r *http.Request) {
	// get problem id
	problemIdStr := r.URL.Query().Get("problem_id")

	// cast it to int
	problemId, err := strconv.Atoi(problemIdStr)
	if err != nil {
		http.Error(w, "invalid problem id, problem id must be an integer", http.StatusBadRequest)
		return
	}

	// fetch the problem using service
	problem, err := a.ProblemServiceConfig.GetProblemById(r.Context(), int32(problemId))
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal the response
	responseBytes, err := json.Marshal(problem)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", responseBytes, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}

func (a *Api) HandlerGetProblemsByFilters(w http.ResponseWriter, r *http.Request) {
	var getProblemsRequest problem_service.GetProblemsRequest
	decodeErr := decodeJsonBody(r.Body, &getProblemsRequest)
	if decodeErr != nil {
		http.Error(w, decodeErr.Error(), http.StatusBadRequest)
		return
	}

	// fetch problems from service
	problems, fetchErr := a.ProblemServiceConfig.GetProblemsByFilters(r.Context(), getProblemsRequest)
	if fetchErr != nil {
		handlerError(fetchErr, w)
		return
	}

	// marshal
	response, marsErr := json.Marshal(problems)
	if marsErr != nil {
		log.Errorf("cannot marshal %v, %v", problems, marsErr)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
