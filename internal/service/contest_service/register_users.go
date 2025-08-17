package contest_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (c *ContestService) RegisterUsersToContest(
	ctx context.Context,
	contestID uuid.UUID,
	userNames []string,
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

	// published contest cannot have any registered users
	if contest.IsPublished {
		return fmt.Errorf(
			"%w, published contests cannot have any registered users",
			flux_errors.ErrInvalidRequest,
		)
	}

	// create a new transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return err
	}

	// if anything goes wrong roll back
	defer tx.Rollback(ctx)

	// get a new query tool with this transaction
	qtx := c.DB.WithTx(tx)

	// unregister previous users
	err = qtx.UnRegisterContestUsers(ctx, contestID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot unregister users of contest with id %v, %w",
			flux_errors.ErrInternal,
			contestID,
			err,
		)
		log.Error(err)
		return err
	}

	// add users to contest
	err = c.addUsersToContest(ctx, qtx, contestID, userNames)
	if err != nil {
		return err
	}

	// commit the transaction if started
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after registering users, %w",
			flux_errors.ErrInternal,
			err,
		)
		return err
	}

	// log if it is a public contest
	if contest.LockId != nil {
		log.Warnf(
			"user %s changed users in public contest with id %v",
			claims.UserName,
			contestID,
		)
	}

	return nil
}
