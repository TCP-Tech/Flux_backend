package user_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (u *UserService) GetUserByUserNameOrRollNo(
	ctx context.Context,
	userName *string,
	rollNo *string,
) (UserMetaData, error) {
	if userName != nil {
		return u.FetchUserByUserName(ctx, *userName)
	} else if rollNo != nil {
		return u.FetchUserByRollNo(ctx, *rollNo)
	}

	return UserMetaData{}, fmt.Errorf(
		"%w, either user_name or roll_no must be provided",
		flux_errors.ErrInvalidRequest,
	)
}

func (u *UserService) FetchUserByUserName(
	ctx context.Context,
	userName string,
) (UserMetaData, error) {
	dbUser, err := u.DB.GetUserByUserName(ctx, userName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err := fmt.Errorf(
				"%w, no user exist with that username",
				flux_errors.ErrNotFound,
			)
			return UserMetaData{}, err
		}
		err = fmt.Errorf(
			"%w, cannot fetch user with name %s from db, %w",
			flux_errors.ErrInternal,
			userName,
			err,
		)
		log.Error(err)
		return UserMetaData{}, err
	}

	return UserMetaData{
		UserID:   dbUser.ID,
		UserName: dbUser.UserName,
		RollNo:   dbUser.RollNo,
	}, nil
}

func (u *UserService) FetchUserByRollNo(
	ctx context.Context,
	rollNo string,
) (UserMetaData, error) {
	dbUser, err := u.DB.GetUserByRollNumber(ctx, rollNo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err := fmt.Errorf(
				"%w, no user exist with that roll_no",
				flux_errors.ErrNotFound,
			)
			return UserMetaData{}, err
		}
		err = fmt.Errorf(
			"%w, cannot fetch user with roll_no %s from db, %w",
			flux_errors.ErrInternal,
			rollNo,
			err,
		)
		log.Error(err)
		return UserMetaData{}, err
	}

	return UserMetaData{
		UserID:   dbUser.ID,
		UserName: dbUser.UserName,
		RollNo:   dbUser.RollNo,
	}, nil
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
	role string,
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

// func (u *UserService) IsUserIDValid(
// 	ctx context.Context,
// 	userID uuid.UUID,
// ) (bool, error) {
// 	exist, err := u.DB.IsUserIDValid(
// 		ctx, userID,
// 	)
// 	if err != nil {
// 		err = fmt.Errorf(
// 			"%w, cannot check if user exist with id %v",
// 			flux_errors.ErrInternal,
// 			userID,
// 		)
// 		log.Error(err)
// 		return false, err
// 	}

// 	return exist, nil
// }
