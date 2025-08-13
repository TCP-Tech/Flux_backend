package api

import (
	"fmt"
	"net/http"
)

func (a *Api) HandlerResetPassword(w http.ResponseWriter, r *http.Request) {
	// get the username and password
	type User struct {
		UserName string `json:"user_name"`
		RollNo   string `json:"roll_no"`
		Password string `json:"password"`
	}
	var user User
	err := decodeJsonBody(r.Body, &user)
	if err != nil {
		msg := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// extract the auth token from header
	verificationToken, err := extractAuthToken(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// reset password
	err = a.AuthServiceConfig.ResetPassword(
		r.Context(),
		user.UserName,
		user.RollNo,
		user.Password,
		verificationToken,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// respond with success
	respondWithJson(w, http.StatusOK, []byte("password reset successful"))
}
