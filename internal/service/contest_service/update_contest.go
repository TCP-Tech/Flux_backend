package contest_service

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (c *ContestService) UpdateContest(
	ctx context.Context,
	contest Contest,
) (Contest, error) {
	// get previous contest
	prevContest, err := c.GetContestByID(ctx, contest.ID)
	if err != nil {
		return Contest{}, err
	}

	// public contests cannot be edited
	if prevContest.LockId != nil {
		return Contest{}, fmt.Errorf(
			"%w, public contests cannot be edited",
			flux_errors.ErrInvalidRequest,
		)
	}

	// authorize
	if err = c.authorizeContestUpdate(ctx, contest); err != nil {
		return Contest{}, err
	}

	// validate the new contest
	if err = c.validatePrivateContest(contest); err != nil {
		return Contest{}, err
	}

	// update the contest
	dbContest, err := c.DB.UpdateContest(
		ctx,
		database.UpdateContestParams{
			Title:     contest.Title,
			StartTime: contest.StartTime,
			EndTime:   contest.EndTime,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, failed update a private contest, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return Contest{}, err
	}

	// add support to return the contest in case
	//  we might allow updating public contests
	return Contest{
		Title:       dbContest.Title,
		ID:          dbContest.ID,
		StartTime:   contest.StartTime,
		EndTime:     dbContest.EndTime,
		CreatedBy:   dbContest.CreatedBy,
		IsPublished: dbContest.IsPublished,
	}, nil
}
