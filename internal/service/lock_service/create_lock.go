package lock_service

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (l *LockService) CreateLock(
	ctx context.Context,
	lock FluxLock,
) (FluxLock, error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return FluxLock{}, err
	}

	// authorize user
	err = l.UserServiceConfig.AuthorizeUserRole(
		ctx,
		user_service.RoleManager,
		fmt.Sprintf("user %s tried to create a lock", claims.UserName),
	)
	if err != nil {
		return FluxLock{}, err
	}

	// validate the lock
	if err = validateLock(lock); err != nil {
		return FluxLock{}, err
	}

	// create the lock
	dbLock, err := l.DB.CreateLock(ctx, database.CreateLockParams{
		Timeout:     lock.Timeout,
		LockType:    lock.Type,
		Name:        lock.Name,
		CreatedBy:   claims.UserId,
		Description: lock.Description,
	})
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot create lock, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return FluxLock{}, err
	}

	return dbLockToServiceLock(dbLock), nil
}
