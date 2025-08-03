package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerUpdateProblem(w http.ResponseWriter, r *http.Request) {
	// get the problem data
	var problem problem_service.Problem
	err := decodeJsonBody(r.Body, &problem)
	if err != nil {
		errorMessage := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}

	// update it using service
	problemResponse, err := a.ProblemServiceConfig.UpdateProblem(r.Context(), problem)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal the response
	responseBytes, err := json.Marshal(problemResponse)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", problemResponse, err)
		http.Error(
			w,
			"problem updated successfully, but there was an error preparing response",
			http.StatusOK,
		)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}
