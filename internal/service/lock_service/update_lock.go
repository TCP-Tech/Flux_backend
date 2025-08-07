package lock_service

import (
	"context"
	"fmt"
	"time"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (l *LockService) UpdateLock(
	ctx context.Context,
	lock FluxLock,
) (res FluxLock, err error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return
	}

	// get previous lock
	previousLock, err := l.GetLockById(ctx, lock.ID)
	if err != nil {
		return
	}

	// authorize
	err = l.UserServiceConfig.AuthorizeCreatorAccess(
		ctx,
		previousLock.CreatedBy,
		claims.UserId,
		fmt.Sprintf(
			"user %s tried to update lock with id %v",
			claims.UserName,
			lock.ID,
		),
	)
	if err != nil {
		return
	}

	if previousLock.Type != lock.Type {
		err = fmt.Errorf(
			"%w, cannot change type of a lock",
			flux_errors.ErrInvalidRequest,
		)
		return
	}

	// check expiry of previous lock
	if previousLock.Type == database.LockTypeTimer &&
		time.Now().After(*previousLock.Timeout) {
		err = fmt.Errorf(
			"%w, lock is already expired, create a new one",
			flux_errors.ErrInvalidRequest,
		)
		return
	}

	// validate new lock
	err = validateLock(lock)
	if err != nil {
		return
	}


	// update the lock
	dbLock, err := l.DB.UpdateLockDetails(
		ctx,
		database.UpdateLockDetailsParams{
			Timeout:     lock.Timeout,
			Description: lock.Description,
			Name:        lock.Name,
			ID:          lock.ID,
		},
	)
	if err != nil {
		// only internal error might occur currently
		err = fmt.Errorf(
			"%w, unable to update lock with id %v, %w",
			flux_errors.ErrInternal,
			lock.ID,
			err,
		)
		return
	}

	return dbLockToServiceLock(dbLock), nil
}
