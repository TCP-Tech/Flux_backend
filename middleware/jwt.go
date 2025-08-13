package middleware

import (
	"context"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/service"
)

/*
	jwt middleware is used to authenticate every endpoint that a user try to access
	this can be used to avoid sending data manually everytime
	a jwt token will be generated once the user logs in.
	while creating a jwt token custom data can be embedded with it
	when the user passses the same token again, we can use it to validate and extract the data
*/

func JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// token will be passed as cookie
		authCookie, err := r.Cookie(KeyJwtSessionCookieName)
		if err != nil {
			if err == http.ErrNoCookie {
				// This is typical if the user is not logged in or their session expired.
				log.Errorf("Error: JWT cookie '%s' not found.\n", KeyJwtSessionCookieName)
				http.Error(
					w, "Authentication required: JWT cookie not found.",
					http.StatusUnauthorized,
				)
				return
			}
			// Other errors, potentially malformed cookie header
			log.Errorf("Error reading JWT cookie '%s': %v\n", KeyJwtSessionCookieName, err)
			http.Error(w, "Bad Request: Error processing cookies.", http.StatusBadRequest)
			return
		}

		tokenString := authCookie.Value

		// get jwt_secret key used during generation of token to parse it back
		jwt_secret := os.Getenv(service.KeyJWTSecret)
		if jwt_secret == "" {
			log.Error("jwt secret key is not found")
			http.Error(
				w, "internal error. please try again later",
				http.StatusInternalServerError,
			)
			return
		}

		// parse the token
		claims := service.UserCredentialClaims{}
		token, err := jwt.ParseWithClaims(
			tokenString, &claims,
			// function to extract the jwt_secret to parse the token
			func(t *jwt.Token) (any, error) {
				// Check the signing method to prevent algorithm confusion
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					log.Errorf("Unexpected signing method: %v", t.Header["alg"])
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwt_secret), nil
			},
		)

		// validate the token
		if err != nil || !token.Valid {
			// The `jwt` library now automatically checks the "exp" claim
			if err != nil {
				// error might be on server side also. log it for safety purpose
				log.Errorf("Invalid Token: %v", err)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// log the endpoint user tyring to access
		log.WithFields(log.Fields{
			"user_name": claims.UserName,
		}).Infof("accessing %v[%v] endpoint", r.Method, r.URL.Path)

		// pass the claims with context
		ctx := context.WithValue(r.Context(), service.KeyCtxUserCredClaims, claims)

		// call the endpoint's handler that the user wants to access
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
