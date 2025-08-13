package user_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

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
	// try to get roles from cache
	roles, ok := u.rolesCache.Get(userId)
	if ok {
		log.Debugf("rolesCache hit for user %v", userId)
		return roles, nil
	}

	// get from db
	log.Debugf("roleCache miss for user %s", userId)
	userRoles, err := u.DB.GetUserRolesByUserName(ctx, userId)
	roles = make([]string, 1)
	roles[0] = "User"

	if err != nil {
		log.Errorf("error fetching roles for user %s, %v", userId, err)
		return nil, flux_errors.ErrInternal
	}
	// convert to string
	for _, userRole := range userRoles {
		roles = append(roles, userRole.RoleName)
	}

	evicted := u.rolesCache.Add(userId, roles)
	log.Debugf("added roles of %v to cache, evicted: %v", userId, evicted)
	return roles, nil
}

func (u *UserService) AuthorizeUserRole(
	ctx context.Context,
	role UserRole,
	warnMessage string,
) (err error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// get roles
	roles, err := u.FetchUserRoles(ctx, claims.UserId)
	if err != nil {
		return err
	}

	// access_role must be present in user_roles
	if slices.Contains(roles, string(role)) {
		return nil
	}

	// warn
	if warnMessage != "" {
		log.Warn(warnMessage)
	}

	return flux_errors.ErrUnAuthorized
}

func (u *UserService) AuthorizeCreatorAccess(
	ctx context.Context,
	creatorId uuid.UUID,
	warnMessage string,
) error {
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// check if they are hc
	err = u.AuthorizeUserRole(
		ctx,
		RoleHC,
		"",
	)
	if err == nil {
		return nil
	}

	if claims.UserId != creatorId {
		return flux_errors.ErrUnAuthorized
	}

	return nil
}

// only 3 functions
// a little duplication is better than a little abstraction
func (u *UserService) IsUserIDValid(
	ctx context.Context,
	userID uuid.UUID,
) (bool, error) {
	exist, err := u.DB.IsUserIDValid(
		ctx, userID,
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot check if user exist with id %v",
			flux_errors.ErrInternal,
			userID,
		)
		log.Error(err)
		return false, err
	}

	return exist, nil
}

func (u *UserService) GetUserIDByUserName(
	ctx context.Context,
	userName string,
) (uuid.UUID, error) {
	userID, err := u.DB.GetUserIDByUserName(ctx, userName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf(
				"%w, user %s does not exist",
				flux_errors.ErrInvalidRequest,
				userName,
			)
			return uuid.Nil, err
		}
		err = fmt.Errorf(
			"%w, cannot fetch userId of user %s, %w",
			flux_errors.ErrInternal,
			userName,
			err,
		)
		log.Error(err)
		return uuid.Nil, err
	}

	return userID, nil
}
