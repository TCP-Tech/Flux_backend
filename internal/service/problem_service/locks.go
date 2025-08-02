package problem_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (p *ProblemService) CreateLock(
	ctx context.Context,
	lock FluxLock,
) (id uuid.UUID, err error) {
	err = validateLock(lock)
	if err != nil {
		return
	}

	// create a lock
	dbLock, err := p.DB.CreateLock(ctx, lock.Timeout)
	if err != nil {
		// error cannot come from client side currently
		err = fmt.Errorf("%w, cannot create lock, %w", flux_errors.ErrInternal, err)
		log.Error(err)
		return
	}

	return dbLock.ID, nil
}
