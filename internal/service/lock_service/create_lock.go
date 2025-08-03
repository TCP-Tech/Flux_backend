package lock_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (l *LockService) CreateLock(
	ctx context.Context,
	lock FluxLock,
) (id uuid.UUID, err error) {
	// get user from claims
	user, err := l.UserServiceConfig.FetchUserFromClaims(ctx)
	if err != nil {
		return
	}

	// authorize user
	err = l.UserServiceConfig.AuthorizeUserRole(
		ctx,
		user.ID,
		user_service.RoleManager,
		fmt.Sprintf("user %s tried to create a lock", user.UserName),
	)
	if err != nil {
		return
	}

	// validate the lock
	err = validateLock(lock)
	if err != nil {
		return
	}

	// create a lock
	dbLock, err := l.DB.CreateLock(ctx, database.CreateLockParams{
		Timeout:     lock.Timeout,
		Name:        lock.Name,
		CreatedBy:   user.ID,
		Description: lock.Description,
	})
	if err != nil {
		// currently error cannot occur from client side
		// while inserting lock in db
		err = fmt.Errorf(
			"%w, cannot create lock, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return
	}

	return dbLock.ID, nil
}
