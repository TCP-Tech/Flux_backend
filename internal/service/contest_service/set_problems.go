package contest_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (c *ContestService) SetProblemsInContest(
	ctx context.Context,
	contestID uuid.UUID,
	problems []ContestProblem,
) error {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// get contest
	contest, err := c.GetContestByID(ctx, contestID)
	if err != nil {
		return err
	}

	// authorize
	err = c.authorizeContestUpdate(ctx, contest)
	if err != nil {
		return err
	}

	// validate the problems
	err = c.validateContestProblems(
		ctx,
		contest.LockId,
		problems,
	)
	if err != nil {
		return err
	}

	// create a new transaction query tool if its nil
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return err
	}

	// if anything goes wrong rollback
	defer tx.Rollback(ctx)

	// get a new query tool with this tx
	qtx := c.DB.WithTx(tx)

	// unset problems
	err = c.unsetContestProblems(ctx, qtx, contestID)
	if err != nil {
		return err
	}

	// add problems
	err = c.addProblemsToContest(
		ctx,
		qtx,
		contestID,
		problems,
	)
	if err != nil {
		return err
	}

	// commit the transaction
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after adding problems, %w",
			flux_errors.ErrInternal,
			err,
		)
		return err
	}

	// log if it is a public contest
	if contest.LockId != nil {
		log.Warnf(
			"user %s changed problems in public contest with id %v",
			claims.UserName,
			contestID,
		)
	}

	return nil
}
