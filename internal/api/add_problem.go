package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerAddProblem(w http.ResponseWriter, r *http.Request) {
	// get the problem body
	var problem problem_service.Problem
	err := decodeJsonBody(r.Body, &problem)
	if err != nil {
		msg := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	serviceProblem, err := a.ProblemServiceConfig.AddProblem(r.Context(), problem)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response_bytes, err := json.Marshal(serviceProblem)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", serviceProblem, err)
		http.Error(
			w,
			"problem added successfully, but there was an error preparing response",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, response_bytes)
}
