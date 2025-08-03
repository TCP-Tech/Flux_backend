package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service/auth_service"
)

func (a *Api) HandlerSignUp(w http.ResponseWriter, r *http.Request) {
	// extract the verification token
	verificationToken, err := extractAuthToken(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// decode the data from body
	var userRegestration auth_service.UserRegestration
	err = decodeJsonBody(r.Body, &userRegestration)
	if err != nil {
		msg := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// try to create the user
	user, err := a.AuthServiceConfig.SignUp(
		r.Context(),
		userRegestration,
		verificationToken,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// response to be sent to the user on success
	response_bytes, err := json.Marshal(user)
	if err != nil {
		log.Errorf("cannot marshal %v. %v", user, err)
		http.Error(
			w,
			"User signed up successfully, but there was an issue preparing the response data. Please try logging in.",
			http.StatusInternalServerError,
		)
		return
	}
	respondWithJson(w, http.StatusCreated, response_bytes)
}
