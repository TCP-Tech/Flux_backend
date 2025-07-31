package api

import (
	"net/http"

	"github.com/tcp_snm/flux/internal/email"
)

func (a *Api) HandlerSignUpSendMail(w http.ResponseWriter, r *http.Request) {
	// extract the email
	queryParams := r.URL.Query()
	userMail := queryParams.Get("email")

	// call the service to verify the email to create a token
	err := a.AuthServiceConfig.SendVerificationEmail(r.Context(), userMail, email.PurposeEmailSignUp)
	if err != nil {
		handlerError(err, w)
		return
	}

	// return if success
	respondWithJson(w, http.StatusOK, []byte("sent verification token to your email. please check once"))
}

func (a *Api) HandlerResetPasswordSendMail(w http.ResponseWriter, r *http.Request) {
	// get the username from query
	queryParams := r.URL.Query()
	userName := queryParams.Get("user_name")
	rollNo := queryParams.Get("roll_no")

	// send mail to reset password
	err := a.AuthServiceConfig.ResetPasswordSendMail(
		r.Context(),
		userName,
		rollNo,
	)
	if err != nil {
		handlerError(err, w)
		return
	}

	// respond with success
	respondWithJson(w, http.StatusOK, []byte("sent verification token to your email. please check once"))
}
