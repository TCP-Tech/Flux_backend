package lock_service

import (
	"fmt"
	"time"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func validateLock(lock FluxLock) error {
	// validate using validator
	err := service.ValidateInput(lock)
	if err != nil {
		return err
	}

	if lock.Type == database.LockTypeTimer {
		return validateTimerLock(lock)
	}

	if lock.Timeout != nil {
		return fmt.Errorf(
			"%w, manual lock cannot have a timer",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}

func validateTimerLock(lock FluxLock) error {
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
	return FluxLock{
		Timeout:     dbLock.Timeout,
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
) bool {
	if lock.Type == database.LockTypeManual {
		return false
	}

	if time.Now().Add(
		time.Minute * time.Duration(delayMinutes)).After(*lock.Timeout) {
		return true
	}

	return false
}
