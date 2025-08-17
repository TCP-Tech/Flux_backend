package lock_service

import (
	"context"
	"fmt"

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
		fmt.Sprintf(
			"user %s tried to update lock with id %v",
			claims.UserName,
			lock.ID,
		),
	)
	if err != nil {
		return
	}

	// validate new lock
	err = l.validateLockUpdate(previousLock, lock)
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

func (l *LockService) validateLockUpdate(
	previousLock FluxLock,
	newLock FluxLock,
) error {
	if previousLock.Type != newLock.Type {
		return fmt.Errorf(
			"%w, cannot change lock's type once created",
			flux_errors.ErrInvalidRequest,
		)
	}

	if previousLock.Type == database.LockTypeTimer {
		return fmt.Errorf(
			"%w, cannot update a timer lock",
			flux_errors.ErrInvalidRequest,
		)
	}

	return validateManualLock(newLock)
}
