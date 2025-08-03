package lock_service

import (
	"context"
	"fmt"
	"time"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (l *LockService) UpdateLock(
	ctx context.Context,
	lock FluxLock,
) (res FluxLock, err error) {
	// fetch user from claims
	user, err := l.UserServiceConfig.FetchUserFromClaims(ctx)
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
		user.ID,
		fmt.Sprintf(
			"user %s tried to update lock with id %v",
			user.UserName,
			lock.ID,
		),
	)
	if err != nil {
		return
	}

	// check expiry of previous lock
	if time.Now().After(previousLock.Timeout) {
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
