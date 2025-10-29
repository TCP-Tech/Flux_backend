package api

import (
	"net/http"
	"time"

	"github.com/tcp_snm/flux/middleware"
)

func (a *Api) HandlerLogout(w http.ResponseWriter, r *http.Request) {
	expiredCookie := &http.Cookie{
		Name:     middleware.KeyJwtSessionCookieName, // must match login cookie name
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0), // expire immediately
		MaxAge:   -1,              // remove cookie right now
		HttpOnly: true,
		Secure:   true, // same as login
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, expiredCookie)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "logged out successfully"}`))
}
