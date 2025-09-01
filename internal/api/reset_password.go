package api

import (
	"net/http"

	"github.com/tcp_snm/flux/internal/service/auth_service"
)

func (a *Api) HandlerResetPassword(w http.ResponseWriter, r *http.Request) {
	var request auth_service.ResetPasswordRequest
	err := decodeJsonBody(r.Body, &request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// reset password
	err = a.AuthServiceConfig.ResetPassword(
		r.Context(),
		request,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// respond with success
	respondWithJson(w, http.StatusOK, []byte("password reset successful"))
}
