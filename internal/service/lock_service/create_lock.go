package lock_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (l *LockService) CreateLock(
	ctx context.Context,
	lock FluxLock,
	generateGroupID bool,
) (res FluxLock, err error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return
	}

	// authorize user
	err = l.UserServiceConfig.AuthorizeUserRole(
		ctx,
		user_service.RoleManager,
		fmt.Sprintf("user %s tried to create a lock", claims.UserName),
	)
	if err != nil {
		return
	}

	// validate the lock
	groupID, err := l.validateLockCreation(ctx, lock, generateGroupID)
	if err != nil {
		return
	}
	if generateGroupID && lock.Type == database.LockTypeManual {
		return lock, fmt.Errorf(
			"%w, cannot create group_id for manual lock",
			flux_errors.ErrInvalidRequest,
		)
	}

	// create a lock
	dbLock, err := l.DB.CreateLock(ctx, database.CreateLockParams{
		Timeout:     lock.Timeout,
		LockType:    lock.Type,
		Name:        lock.Name,
		CreatedBy:   claims.UserId,
		Description: lock.Description,
		GroupID:     groupID,
	})
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot create lock, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return
	}

	return dbLockToServiceLock(dbLock), nil
}

func (l *LockService) validateLockCreation(
	ctx context.Context,
	lock FluxLock,
	generateGroupId bool,
) (*uuid.UUID, error) {
	if generateGroupId {
		// cannot create group id for a manual lock
		if lock.Type == database.LockTypeManual {
			return nil, fmt.Errorf(
				"%w, cannot generate group_id for manual locks",
				flux_errors.ErrInvalidRequest,
			)
		}

		// group id must be null to create a timer lock
		if lock.GroupID != nil {
			return nil, fmt.Errorf(
				"%w, group_id must be null in the request to generate a random group_id",
				flux_errors.ErrInvalidRequest,
			)
		}

		// validate the timer lock
		err := validateTimerLockTimeout(lock)
		if err != nil {
			return nil, err
		}

		// generate a new groupId
		groupId := uuid.New()
		return &groupId, nil
	}

	err := l.validateLock(ctx, lock)
	if err != nil {
		return nil, err
	}

	return lock.GroupID, nil
}
