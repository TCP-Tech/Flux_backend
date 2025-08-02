package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (a *Api) HandlerCreateLock(w http.ResponseWriter, r *http.Request) {
	var lock problem_service.FluxLock
	err := decodeJsonBody(r.Body, &lock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := a.ProblemServiceConfig.CreateLock(r.Context(), lock)
	if err != nil {
		handlerError(err, w)
		return
	}

	response := struct {
		ID uuid.UUID `json:"lock_id"`
	}{ID: id}
	bytes, err := json.Marshal(response)
	if err != nil {
		log.Errorf("error marshalling response, but lock created with ID %s: %v", id.String(), err)
		http.Error(
			w,
			fmt.Sprintf("Failed to prepare response, but lock was created with ID: %s", id.String()),
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusCreated, bytes)
}
