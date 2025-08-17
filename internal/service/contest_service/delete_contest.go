package contest_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (c *ContestService) DeleteContest(
	ctx context.Context,
	id uuid.UUID,
) error {
	// get previous contest
	prevContest, err := c.GetContestByID(ctx, id)
	if err != nil {
		return err
	}

	// public contests cannot be deleted
	if prevContest.LockId != nil {
		return fmt.Errorf(
			"%w, cannot delete a public contest",
			flux_errors.ErrInvalidRequest,
		)
	}

	// authorize
	err = c.authorizeContestUpdate(ctx, prevContest)
	if err != nil {
		return err
	}

	// start a new transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return err
	}

	// if anything goes wrong
	defer tx.Rollback(ctx)

	// get a new query tool with the transaction
	qtx := c.DB.WithTx(tx)

	// unregister users
	err = qtx.UnRegisterContestUsers(ctx, id)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot unregister users of contest %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		log.Error(err)
		return err
	}

	// delete contest problems
	err = qtx.DeleteProblemsByContestId(ctx, id)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot delete problems of contest %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		log.Error(err)
		return err
	}

	// delete contest
	err = qtx.DeleteContestByID(ctx, id)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot delete contest %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		log.Error(err)
		return err
	}

	// commit the tx
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction while deleting the contest %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		log.Error(err)
		return err
	}

	return nil
}
