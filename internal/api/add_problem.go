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

	problem_id, err := a.ProblemServiceConfig.AddProblem(r.Context(), problem)
	if err != nil {
		handlerError(err, w)
		return
	}

	response := struct {
		ProblemId int32 `json:"problem_id"`
	}{problem_id}
	response_bytes, err := json.Marshal(response)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", response, err)
		respondWithJson(w, http.StatusOK, []byte("problem added successfully, but there was an error sending response"))
		return
	}

	respondWithJson(w, http.StatusOK, response_bytes)
}