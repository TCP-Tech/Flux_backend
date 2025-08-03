package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (a *Api) HandlerGetLocksByFilter(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("lock_id")
	if idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			http.Error(w, "invalid lock_id provided", http.StatusBadRequest)
			return
		}
		a.getLockById(r.Context(), id, w)
		return
	}

	// get the title
	lockName := r.URL.Query().Get("lock_name")

	// get the creator details
	creatorUserName := r.URL.Query().Get("creator_user_name")
	creatorRollNo := r.URL.Query().Get("creator_roll_no")

	// get locks using service
	locks, err := a.LockServiceConfig.GetLocksByFilters(
		r.Context(),
		lockName,
		creatorUserName,
		creatorRollNo,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	bytes, err := json.Marshal(locks)
	if err != nil {
		log.Errorf("failed to marshal %v, %v", locks, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, bytes)
}

func (a *Api) getLockById(ctx context.Context, id uuid.UUID, w http.ResponseWriter) {
	lock, err := a.LockServiceConfig.GetLockById(ctx, id)
	if err != nil {
		handlerError(err, w)
		return
	}

	responseBytes, err := json.Marshal(lock)
	if err != nil {
		log.Errorf("unable to marshal %v, %v", lock, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, responseBytes)
}
