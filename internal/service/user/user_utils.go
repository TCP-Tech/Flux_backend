package flux_user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (u *UserService) FetchUserFromDb(
	ctx context.Context,
	userName string,
	rollNo string,
) (dbUser database.User, err error) {
	if userName != "" {
		dbUser, err = u.FetchUserByUserName(ctx, userName)
	} else {
		dbUser, err = u.FetchUserByRollNo(ctx, rollNo)
	}
	return
}

func (u *UserService) FetchUserByUserName(
	ctx context.Context,
	userName string,
) (user database.User, err error) {
	user, dbErr := u.DB.GetUserByUserName(ctx, userName)
	if dbErr != nil {
		if errors.Is(dbErr, sql.ErrNoRows) {
			err = fmt.Errorf("%w, no user exist with that username", flux_errors.ErrInvalidUserCredentials)
			return
		}
		log.Errorf("failed to get user by username. %v", dbErr)
		err = errors.Join(flux_errors.ErrInternal, dbErr)
		return
	}
	return
}

func (u *UserService) FetchUserByRollNo(
	ctx context.Context,
	rollNo string,
) (user database.User, err error) {
	user, dbErr := u.DB.GetUserByRollNumber(ctx, rollNo)
	if dbErr != nil {
		if errors.Is(dbErr, sql.ErrNoRows) {
			err = fmt.Errorf("%w, no user exist with that roll_no", flux_errors.ErrInvalidUserCredentials)
			return
		}
		log.Errorf("failed to get user by roll number. %v", dbErr)
		err = errors.Join(dbErr, flux_errors.ErrInternal)
		return
	}
	return
}

// extract user roles
func (u *UserService) FetchUserRoles(ctx context.Context, userId uuid.UUID) ([]string, error) {
	userRoles, err := u.DB.GetUserRolesByUserName(ctx, userId)
	roles := make([]string, 1)
	roles[0] = "User"

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return roles, nil
		}
		log.Errorf("error fetching roles for user %s, %v", userId, err)
		return nil, flux_errors.ErrInternal
	}
	for _, userRole := range userRoles {
		roles = append(roles, userRole.RoleName)
	}

	return roles, nil
}

func (u *UserService) AuthorizeManager(ctx context.Context, userId uuid.UUID) error {
	roles, err := u.FetchUserRoles(ctx, userId)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role == "role_manager" {
			return nil
		}
	}
	return flux_errors.ErrUnAuthorized
}

func (u *UserService) FetchUserFromClaims(ctx context.Context) (user database.User, err error) {
	claimsValue := ctx.Value(service.KeyCtxUserCredClaims)
	claims, ok := claimsValue.(service.UserCredentialClaims)
	if !ok {
		err = fmt.Errorf(
			"%w, unable to parse claims to auth_service.UserCredentialClaims, type of claims found is %T",
			flux_errors.ErrInternal,
			reflect.TypeOf(claims),
		)
		return
	}
	// fetch user from db
	user, err = u.FetchUserFromDb(ctx, claims.UserName, claims.RollNo)
	return
}
