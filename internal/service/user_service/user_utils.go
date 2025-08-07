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
)

func (u *UserService) FetchUserFromDb(
	ctx context.Context,
	userName string,
	rollNo string,
) (dbUser database.User, err error) {
	if userName == "" && rollNo == "" {
		err = fmt.Errorf("%w, either user_name or roll_no must be provided", flux_errors.ErrInvalidRequest)
		return
	}
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
	userId uuid.UUID,
	role UserRole,
	warnMessage string,
) (err error) {
	roles, err := u.FetchUserRoles(ctx, userId)
	if err != nil {
		return err
	}
	if slices.Contains(roles, string(role)) {
		return nil
	}
	if warnMessage != "" {
		log.Warn(warnMessage)
	}
	return flux_errors.ErrUnAuthorized
}

func (u *UserService) AuthorizeCreatorAccess(
	ctx context.Context,
	creatorId uuid.UUID,
	userId uuid.UUID,
	warnMessage string,
) error {
	// check if they are hc
	err := u.AuthorizeUserRole(
		ctx,
		userId,
		RoleHC,
		"",
	)
	if err == nil {
		return nil
	}

	// check if they are manager currently
	err = u.AuthorizeUserRole(
		ctx,
		userId,
		RoleManager,
		warnMessage,
	)
	if err != nil {
		return err
	}

	if userId != creatorId {
		return flux_errors.ErrUnAuthorized
	}

	return nil
}
