package lock_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (l *LockService) validateLock(
	ctx context.Context,
	lock FluxLock,
) error {
	// validate using validator
	err := service.ValidateInput(lock)
	if err != nil {
		return err
	}

	if lock.Type == database.LockTypeManual {
		return validateManualLock(lock)
	}

	if lock.GroupID == nil {
		return fmt.Errorf(
			"%w, timer lock must have a group id",
			flux_errors.ErrInvalidRequest,
		)
	}

	if lock.Timeout == nil {
		return fmt.Errorf(
			"%w, timer lock must have non-null timeout",
			flux_errors.ErrInvalidRequest,
		)
	}

	// get lock group's timeout
	timeout, err := l.DB.GetLockGroupTimeout(ctx, lock.GroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(
				"%w, no locks exist with that group id",
				flux_errors.ErrInvalidRequest,
			)
		}
		err = fmt.Errorf(
			"%w, cannot fetch lock group timeout with id %v, %w",
			flux_errors.ErrInternal,
			lock.GroupID,
			err,
		)
		log.Error(err)
		return err
	}

	if timeout == nil {
		err = fmt.Errorf(
			"%w, lock group with id %v has timeout nil",
			flux_errors.ErrInternal,
			lock.GroupID,
		)
		return err
	}

	if (*timeout).UTC() != (*lock.Timeout).UTC() {
		return fmt.Errorf(
			"%w, other locks in that group id have timeout as %v",
			flux_errors.ErrInvalidRequest,
			(*timeout).UTC(),
		)
	}

	return nil
}

func validateManualLock(lock FluxLock) error {
	if lock.Timeout != nil {
		return fmt.Errorf(
			"%w, manual lock cannot have a timer",
			flux_errors.ErrInvalidRequest,
		)
	}

	if lock.GroupID != nil {
		return fmt.Errorf(
			"%w, manual lock cannot have a group id",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}

func validateTimerLockTimeout(lock FluxLock) error {
	if lock.Timeout == nil {
		return fmt.Errorf(
			"%w, timer lock must not have timeout as null. Try to check the format of the timout",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate format
	if lock.Timeout.Equal(time.Time{}) {
		return fmt.Errorf(
			"%w, lock's timeout format might be invalid, please check it once",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate expiry
	if time.Now().Add(time.Minute * 5).After(*lock.Timeout) {
		return fmt.Errorf(
			"%w, lock's expiry must be atleast 5 minutes from now",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil

}

func dbLockToServiceLock(dbLock database.Lock) FluxLock {
	var timeout *time.Time
	if dbLock.Timeout != nil {
		utc := (*dbLock.Timeout).UTC()
		timeout = &utc
	}
	return FluxLock{
		Timeout:     timeout,
		CreatedBy:   dbLock.CreatedBy,
		CreatedAt:   dbLock.CreatedAt,
		Name:        dbLock.Name,
		ID:          dbLock.ID,
		Description: dbLock.Description,
		Type:        dbLock.LockType,
		Access:      user_service.UserRole(dbLock.Access),
		GroupID:     dbLock.GroupID,
	}
}

func (l *LockService) IsLockExpired(
	lock FluxLock,
	delayMinutes int32,
) (bool, error) {
	if lock.Type == database.LockTypeManual {
		return false, nil
	}

	// very rare, but for safety purpose
	if lock.Timeout == nil {
		return false, fmt.Errorf(
			"%w, timer lock has timeout as nil",
			flux_errors.ErrInternal,
		)
	}

	if time.Now().Add(
		time.Minute * time.Duration(delayMinutes)).After(*lock.Timeout) {
		return true, nil
	}

	return false, nil
}

func (l *LockService) AuthorizeLock(
	ctx context.Context,
	timeout *time.Time,
	access user_service.UserRole,
	warnMessage string,
) error {
	// timer lock expired
	if timeout != nil {
		if time.Now().After(*timeout) {
			return nil
		}
	}

	// authorize
	err := l.UserServiceConfig.AuthorizeUserRole(
		ctx,
		access,
		warnMessage,
	)

	return err
}
