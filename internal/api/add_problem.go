package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerAddStandardProblem(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Problem             problem_service.Problem             `json:"problem"`
		StandardProblemData problem_service.StandardProblemData `json:"problem_data"`
	}

	// decode from body
	var params Params
	err := decodeJsonBody(r.Body, &params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create the problem
	problemResponse, spdResponse, err := a.ProblemServiceConfig.AddStandardProblem(
		r.Context(),
		params.Problem,
		params.StandardProblemData,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response := Params{
		Problem:             problemResponse,
		StandardProblemData: spdResponse,
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			response,
			err,
		)
		log.Error(err)
		http.Error(w, "problem created, but error in preparing response", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusCreated, responseBytes)
}
