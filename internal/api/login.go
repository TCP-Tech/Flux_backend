package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/middleware"
)

func (a *Api) HandlerLogin(w http.ResponseWriter, r *http.Request) {
	// extract user details for login
	type params struct {
		UserName         string `json:"user_name"`
		RollNo           string `json:"roll_no"`
		Password         string `json:"password"`
		RememberForMonth bool   `json:"remember_for_month"`
	}
	var param params

	// decode from the json body
	err := decodeJsonBody(r.Body, &param)
	if err != nil {
		msg := fmt.Sprintf("invalid request payload, %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// validate the user and gen a jwt token
	userLoginResponse, jwtToken, tokenExpiry, err := a.AuthServiceConfig.Login(
		r.Context(),
		param.UserName,
		param.RollNo,
		param.Password,
		param.RememberForMonth,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	responseBytes, err := json.Marshal(userLoginResponse)
	if err != nil {
		log.WithField("resonse", userLoginResponse).Errorf("unable to marshal login response %v", err)
		http.Error(w, "internal error. please try again later", http.StatusInternalServerError)
		return
	}

	// set jwt session cookie
	cookie := &http.Cookie{
		Name:     middleware.KeyJwtSessionCookieName,
		Value:    jwtToken,
		Expires:  tokenExpiry,
		Path:     "/",                  // Important: Makes the cookie available across the entire site
		HttpOnly: true,                 // Crucial: Prevents JavaScript access
		Secure:   true,                 // Crucial: Only send over HTTPS
		SameSite: http.SameSiteLaxMode, // Recommended: Protects against CSRF
	}
	http.SetCookie(w, cookie)

	log.WithFields(log.Fields{
		"user_name": userLoginResponse.UserName,
		"roll_no":   userLoginResponse.RollNo,
	}).Info("logged in")

	respondWithJson(w, http.StatusOK, responseBytes)
}
