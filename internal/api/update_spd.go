package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerUpdateSpd(w http.ResponseWriter, r *http.Request) {
	// get the problem data
	var spd problem_service.StandardProblemData
	err := decodeJsonBody(r.Body, &spd)
	if err != nil {
		errorMessage := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}

	// update it using service
	spdResponse, err := a.ProblemServiceConfig.UpdateStandardProblem(r.Context(), spd)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal the response
	responseBytes, err := json.Marshal(spdResponse)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", spdResponse, err)
		http.Error(
			w,
			"spd updated successfully, but there was an error preparing response",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}
