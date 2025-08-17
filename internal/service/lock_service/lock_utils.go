package lock_service

import (
	"context"
	"fmt"
	"time"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func validateLock(lock FluxLock) error {
	if lock.Type == database.LockTypeManual {
		return validateManualLock(lock)
	}

	return validateTimerLockTimeout(lock)
}

func validateManualLock(lock FluxLock) error {
	// raw validation
	err := service.ValidateInput(lock)
	if err != nil {
		return err
	}

	if lock.Timeout != nil {
		return fmt.Errorf(
			"%w, manual lock cannot have a timer",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}

func validateTimerLockTimeout(lock FluxLock) error {
	// raw validation
	err := service.ValidateInput(lock)
	if err != nil {
		return err
	}

	if lock.Timeout == nil {
		return fmt.Errorf(
			"%w, timer lock must have a timeout",
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
	if time.Now().After(*lock.Timeout) {
		return fmt.Errorf(
			"%w, timer lock's timeout is in the past, please set a future time",
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
