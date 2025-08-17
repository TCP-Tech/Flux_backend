package contest_service

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (c *ContestService) CreateContest(
	ctx context.Context,
	request CreateContestRequest,
) (Contest, error) {
	// start time is specified for private contest
	// but must be extracted from lock for public contest
	var startTime *time.Time

	// contest details validations
	if request.ContestDetails.LockId == nil {
		err := c.validatePrivateContest(request.ContestDetails)
		if err != nil {
			return Contest{}, err
		}
		// validation ensure startTime isnt nil
		startTime = request.ContestDetails.StartTime
	} else {
		// get lock here to get start time from its expiry
		lock, err := c.LockServiceConfig.GetLockById(
			ctx,
			*request.ContestDetails.LockId,
		)
		if err != nil {
			return Contest{}, err
		}

		// validate the public contest
		err = c.validatePublicContest(request, lock)
		if err != nil {
			return Contest{}, err
		}

		// set the startTime as the expiry of the lock
		startTime = lock.Timeout
	}

	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Contest{}, err
	}

	// validate problems
	err = c.validateContestProblems(
		ctx,
		request.ContestDetails.LockId,
		request.ContestProblems,
	)
	if err != nil {
		return Contest{}, err
	}

	// start a transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return Contest{}, err
	}

	// if anything goes wrong roll back transaction
	defer tx.Rollback(ctx)

	// get a new query tool with this transaction
	qtx := c.DB.WithTx(tx)

	// create contest
	dbContest, err := qtx.CreateContest(
		ctx,
		database.CreateContestParams{
			Title:       request.ContestDetails.Title,
			CreatedBy:   claims.UserId,
			StartTime:   request.ContestDetails.StartTime,
			EndTime:     request.ContestDetails.EndTime,
			IsPublished: request.ContestDetails.IsPublished,
			LockID:      request.ContestDetails.LockId,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot create contest, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.WithField("contest details", request.ContestDetails).Error(err)
		return Contest{}, err
	}

	log.Info(dbContest.ID)

	// add problems
	err = c.addProblemsToContest(
		ctx,
		qtx,
		dbContest.ID,
		request.ContestProblems,
	)
	if err != nil {
		return Contest{}, err
	}

	// add users
	err = c.addUsersToContest(ctx, qtx, dbContest.ID, request.RegisteredUsers)
	if err != nil {
		return Contest{}, err
	}

	// commit the transaction
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after creating contest, %w",
			flux_errors.ErrInternal,
			err,
		)
		return Contest{}, err
	}

	// prepare response and return
	utcStartTime := startTime.UTC()
	return Contest{
		ID:          dbContest.ID,
		Title:       dbContest.Title,
		LockId:      dbContest.LockID,
		StartTime:   &utcStartTime,
		EndTime:     dbContest.EndTime.UTC(),
		IsPublished: dbContest.IsPublished,
		CreatedBy:   dbContest.CreatedBy,
	}, nil
}
