package auth_service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func dbUserToUserCredClaims(
	expirationTime time.Time,
	user database.User,
) service.UserCredentialClaims {
	return service.UserCredentialClaims{
		UserId:   user.ID,
		UserName: user.UserName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime), // Set the expiration time
			IssuedAt:  jwt.NewNumericDate(time.Now()),     // When the token was issued
			Subject:   "user_authentication",              // Purpose of the token
			Issuer:    "flux-auth-service",                // Who issued the token
		},
	}
}

func dbUserToLoginRes(roles []string, dbUser database.User) UserLoginResponse {
	return UserLoginResponse{
		UserName:  dbUser.UserName,
		RollNo:    dbUser.RollNo,
		FirstName: dbUser.FirstName,
		LastName:  dbUser.LastName,
		Roles:     roles,
	}
}

func userRegToResponse(user_name string, userReg UserRegestration) UserRegestrationResponse {
	return UserRegestrationResponse{
		UserName: user_name,
		RollNo:   userReg.RollNo,
	}
}

func generateToken() (string, error) {
	tokenBytes := make([]byte, 9)

	// Read random data from the cryptographically secure source.
	_, err := rand.Read(tokenBytes)
	if err != nil {
		// A failure here is a serious issue, as it means we can't
		// generate a secure token.
		log.Errorf("failed to generate random bytes: %v", err)
		return "", errors.Join(flux_errors.ErrInternal, err)
	}

	// Encode the random bytes to a URL-safe base64 string.
	// This makes the token safe to include in URLs and emails.
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	return token, nil

}

// hashPassword encapsulates the password hashing logic.
func generatePasswordHash(password string) (string, error) {
	passwordHash, bcErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if bcErr != nil {
		log.Errorf("password hash cannot be created: %v", bcErr)
		return "", errors.Join(flux_errors.ErrInternal, bcErr)
	}
	return string(passwordHash), nil
}
