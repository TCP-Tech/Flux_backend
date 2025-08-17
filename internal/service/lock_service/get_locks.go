package lock_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (l *LockService) GetLockById(
	ctx context.Context,
	id uuid.UUID,
) (res FluxLock, err error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return
	}

	// get lock from db
	dbLock, err := l.DB.GetLockById(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf(
				"%w, lock with given id do not exist",
				flux_errors.ErrNotFound,
			)
			return
		}
		err = fmt.Errorf(
			"%w, cannot fetch lock with id %v from db, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		return
	}

	// authorize
	err = l.AuthorizeLock(
		ctx,
		dbLock.Timeout,
		user_service.UserRole(dbLock.Access),
		fmt.Sprintf(
			"user %s tried to view lock with id %v",
			claims.UserName,
			id,
		),
	)
	if err != nil {
		if errors.Is(err, flux_errors.ErrUnAuthorized) {
			err = fmt.Errorf(
				"%w, lock with given id does not exist",
				flux_errors.ErrNotFound,
			)
		}
		return
	}

	return dbLockToServiceLock(dbLock), nil
}

func (l *LockService) GetLocksByFilters(
	ctx context.Context,
	request GetLocksRequest,
) ([]FluxLock, error) {
	// validate request
	err := service.ValidateInput(request)
	if err != nil {
		return nil, err
	}

	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// authorize
	// only manager or above can view locks
	err = l.UserServiceConfig.AuthorizeUserRole(
		ctx,
		user_service.RoleManager,
		fmt.Sprintf(
			"user %s tried to view lock with filters",
			claims.UserName,
		),
	)
	if err != nil {
		return nil, err
	}

	// fetch creator id if user_name or roll_no is provided
	var createdBy *uuid.UUID
	if request.CreatorUserName != "" || request.CreatorRollNo != "" {
		user, err := l.UserServiceConfig.GetUserByUserNameOrRollNo(
			ctx,
			request.CreatorUserName,
			request.CreatorRollNo,
		)
		if err != nil {
			return nil, err
		}
		createdBy = &user.ID
	}

	// calculate offset
	offset := (request.PageNumber - 1) * request.PageSize

	// fetch the locks by filters
	dbLocks, err := l.DB.GetLocksByFilter(
		ctx,
		database.GetLocksByFilterParams{
			LockName:  request.LockName,
			CreatedBy: createdBy,
			Offset:    offset,
			Limit:     request.PageSize,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch locks from db, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.WithField("request", request).Error(err)
		return nil, err
	}

	// convert the locks to service locks
	locks := make([]FluxLock, 0, len(dbLocks))
	for _, dbLock := range dbLocks {
		err = l.AuthorizeLock(
			ctx,
			dbLock.Timeout,
			user_service.UserRole(dbLock.Access),
			"",
		)
		if err != nil {
			log.Debug(err)
		}
		locks = append(locks, dbLockToServiceLock(dbLock))
	}

	return locks, nil
}
