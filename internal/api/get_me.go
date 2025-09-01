package api

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (a *Api) HandlerGetMe(w http.ResponseWriter, r *http.Request) {
	user, err := a.UserServiceConfig.GetMe(r.Context())
	if err != nil {
		handlerError(err, w)
		return
	}

	// marshal
	response, err := json.Marshal(user)
	if err != nil {
		log.Errorf("cannot marshal %s, %v", user, err)
		http.Error(w, flux_errors.ErrInternal.Error(), http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, response)
}
