package lock_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (l *LockService) DeleteLock(ctx context.Context, lockId uuid.UUID) error {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// get the lock
	lock, err := l.GetLockById(ctx, lockId)
	if err != nil {
		return err
	}

	// authorize
	err = l.UserServiceConfig.AuthorizeCreatorAccess(
		ctx,
		lock.CreatedBy,
		fmt.Sprintf(
			"user %s tried to delete lock with id %v",
			claims.UserName,
			lock.ID,
		),
	)
	if err != nil {
		return err
	}

	// delete the lock
	if lock.Type == database.LockTypeTimer {
		return fmt.Errorf(
			"%w, timer lock cannot be deleted once created",
			flux_errors.ErrInvalidRequest,
		)
	}
	err = l.DB.DeleteLockById(ctx, lockId)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			"lock cannot be deleted",
		)
		return err
	}

	return nil
}
