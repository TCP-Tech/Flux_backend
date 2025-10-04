package api

import (
	"fmt"
	"net/http"

	"github.com/tcp_snm/flux/internal/service/submission_service"
)

func (a *Api) HandlerSubmit(w http.ResponseWriter, r *http.Request) {
	var request submission_service.SubmissionRequest

	if err := decodeJsonBody(r.Body, &request); err != nil {
		msg := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := a.SubmissionService.Submit(r.Context(), request); err != nil {
		handlerError(err, w)
		return
	}

	respondWithJson(w, http.StatusOK, []byte("submitted successfully"))
}
