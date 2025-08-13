package auth_service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func (a *AuthService) Login(
	ctx context.Context,
	userName string,
	rollNo string,
	password string,
	rememberForMonth bool,
) (userLoginResponse UserLoginResponse, tokenString string, tokenExpiry time.Time, err error) {
	// get user from db
	user, err := a.UserConfig.GetUserByUserNameOrRollNo(ctx, userName, rollNo)
	if err != nil {
		return
	}

	// validate the password
	bcErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if bcErr != nil {
		if errors.Is(bcErr, bcrypt.ErrMismatchedHashAndPassword) {
			err = flux_errors.ErrInvalidUserCredentials
			return
		}
		log.Errorf("failed to login user. %v", bcErr)
		err = errors.Join(flux_errors.ErrInternal, bcErr)
		return
	}

	// fetch user roles to store in claims
	roles, err := a.UserConfig.FetchUserRoles(ctx, user.ID)
	if err != nil {
		return
	}

	// generate a jwt token
	var duration = time.Hour * 24
	if rememberForMonth {
		duration *= 30
	}
	var exp = time.Now().Add(duration)

	// claims store the user data to avoid repeated logins via jwt
	claims := dbUserToUserCredClaims(exp, user)
	tokenString, err = GenerateJWT(claims)
	if err != nil {
		return
	}

	tokenExpiry = exp
	userLoginResponse = dbUserToLoginRes(roles, user)

	return
}

func GenerateJWT(claims service.UserCredentialClaims) (tokenString string, err error) {
	// generate a token with a signing method and claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// get the jwt secret to generate a token string
	jwtKey := os.Getenv("JWT_SECRET")
	if jwtKey == "" {
		log.Errorf("jwt secret key not found")
		err = fmt.Errorf("%w, jwt secret key not found", flux_errors.ErrInternal)
		return
	}

	// generate a jwt token
	tokenString, signErr := token.SignedString([]byte(jwtKey))
	if signErr != nil {
		// almost always sign error is cause by internal issues
		log.Error(signErr)
		err = errors.Join(flux_errors.ErrInternal, signErr)
	}

	return
}
