package tournament_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (t *TournamentService) validateTournamentRoundLock(
	ctx context.Context,
	lockID uuid.UUID,
) error {
	// get the lock
	lock, err := t.LockServiceConfig.GetLockById(ctx, lockID)
	if err != nil {
		return err
	}

	// check its type
	if lock.Type != database.LockTypeManual {
		return fmt.Errorf(
			"%w, can only use manual locks for tournament round creation",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}
