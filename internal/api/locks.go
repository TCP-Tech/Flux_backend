package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/lock_service"
)

func (a *Api) HandlerCreateLock(w http.ResponseWriter, r *http.Request) {
	// decode the body
	var lock lock_service.FluxLock
	err := decodeJsonBody(r.Body, &lock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// create lock with service
	serviceLock, err := a.LockServiceConfig.CreateLock(
		r.Context(),
		lock,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	bytes, err := json.Marshal(serviceLock)
	if err != nil {
		log.Errorf(
			"cannot marshal %v, %v",
			lock,
			err,
		)
		http.Error(
			w,
			fmt.Sprintf(
				"Failed to prepare response, but lock was created with ID: %v",
				lock.ID,
			),
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusCreated, bytes)
}

func (a *Api) HandlerUpdateLock(w http.ResponseWriter, r *http.Request) {
	// get the data
	var currentLock lock_service.FluxLock
	err := decodeJsonBody(r.Body, &currentLock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// update the lock
	updatedLock, err := a.LockServiceConfig.UpdateLock(
		r.Context(),
		currentLock,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	bytes, err := json.Marshal(updatedLock)
	if err != nil {
		log.Error(err)
		http.Error(
			w,
			"lock was updated, but there was an error preparing response",
			http.StatusInternalServerError,
		)
		return
	}

	respondWithJson(w, http.StatusOK, bytes)
}

func (a *Api) HanlderDeleteLockById(w http.ResponseWriter, r *http.Request) {
	// get the id
	lockIdStr := r.URL.Query().Get("lock_id")
	lockId, err := uuid.Parse(lockIdStr)
	if err != nil {
		http.Error(w, "invalid lock id provided", http.StatusBadRequest)
		return
	}

	err = a.LockServiceConfig.DeleteLock(r.Context(), lockId)
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("lock deleted successfully"))
}
