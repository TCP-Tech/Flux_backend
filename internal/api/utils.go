package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

// extractAuthToken extracts the Bearer token from the Authorization header.
// Returns the token string if present and well-formed, otherwise returns an error.
func extractAuthToken(header http.Header) (string, error) {
	tokenHeader := header.Get("Authorization")
	if tokenHeader == "" {
		errorMessage := "verification token header not found"
		log.Error(errorMessage)
		return "", fmt.Errorf("%w, %s", flux_errors.ErrInvalidRequestCredentials, errorMessage)
	}

	tokenParts := strings.SplitN(tokenHeader, " ", 2)
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		errorMessage := "malformed verification token header"
		log.Error(errorMessage)
		return "", fmt.Errorf("%w, %s", flux_errors.ErrInvalidRequestCredentials, errorMessage)
	}

	return tokenParts[1], nil
}

func handlerError(err error, w http.ResponseWriter) {
	if err != nil {
		var statusCode int
		responseMessage := err.Error()
		switch {
		case errors.Is(err, flux_errors.ErrVerificationTokenExpired):
			fallthrough
		case errors.Is(err, flux_errors.ErrInvalidRequest):
			statusCode = http.StatusBadRequest
		case errors.Is(err, flux_errors.ErrNotFound):
			statusCode = http.StatusNotFound
		case errors.Is(err, flux_errors.ErrUserAlreadyExists):
			statusCode = http.StatusConflict
		case errors.Is(err, flux_errors.ErrCorruptedVerification):
			fallthrough
		case errors.Is(err, flux_errors.ErrInvalidRequestCredentials):
			fallthrough
		case errors.Is(err, flux_errors.ErrUnAuthorized):
			fallthrough
		case errors.Is(err, flux_errors.ErrInvalidUserCredentials):
			statusCode = http.StatusUnauthorized
		case errors.Is(err, flux_errors.ErrEmailServiceStopped):
			fallthrough
		default:
			statusCode = http.StatusInternalServerError
			responseMessage = "internal error. please try again later"
		}
		http.Error(w, responseMessage, statusCode)
		return
	}
}
