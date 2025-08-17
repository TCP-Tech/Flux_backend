package api

import (
	"net/http"

	"github.com/google/uuid"
)

func (a *Api) HanlderDeleteContest(w http.ResponseWriter, r *http.Request) {
	// get the id
	contestIDStr := r.URL.Query().Get("contest_id")
	
	// parse
	contestID, err := uuid.Parse(contestIDStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// delete contest using service
	err = a.ContestServiceConfig.DeleteContest(r.Context(), contestID)
	if err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("contest deleted successfully"))
}