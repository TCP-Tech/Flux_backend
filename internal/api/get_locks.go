package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/lock_service"
)

func (a *Api) HandlerGetLockById(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("lock_id")

	// parse id
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid lock_id provided", http.StatusBadRequest)
		return
	}

	// get lock from service
	lock, err := a.LockServiceConfig.GetLockById(r.Context(), id)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	responseBytes, err := json.Marshal(lock)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", lock, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	// respond
	respondWithJson(w, http.StatusOK, responseBytes)
}

func (a *Api) HandlerGetLocksByFilter(w http.ResponseWriter, r *http.Request) {
	// decode request from body
	var getLockRequest lock_service.GetLocksRequest
	err := decodeJsonBody(r.Body, &getLockRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get locks
	locks, err := a.LockServiceConfig.GetLocksByFilters(
		r.Context(),
		getLockRequest,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(locks)
	if err != nil {
		log.Errorf("cannot marshal %v, %v", locks, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
