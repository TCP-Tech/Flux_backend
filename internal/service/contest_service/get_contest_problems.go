package contest_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (c *ContestService) GetContestProblems(
	ctx context.Context,
	contestID uuid.UUID,
) ([]ContestProblemResponse, error) {
	// get the contest
	contest, err := c.GetContestByID(ctx, contestID)
	if err != nil {
		return nil, err
	}

	// authorize
	err = c.authorizeProblemView(ctx, contest)
	if err != nil {
		return nil, err
	}

	// get the problem ids from db
	dbProblems, err := c.DB.GetContestProblemsByContestID(ctx, contest.ID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch problems of contest with id %v from db, %w",
			flux_errors.ErrInternal,
			contest.ID,
			err,
		)
		log.Error(err)
		return nil, err
	}

	// make a result struct
	res := make([]ContestProblemResponse, 0, len(dbProblems))
	if len(dbProblems) == 0 {
		return res, nil
	}

	// extract id from problems
	problemIDs := make([]int32, 0, len(dbProblems))
	for _, dbProblem := range dbProblems {
		problemIDs = append(problemIDs, dbProblem.ProblemID)
	}

	// fetch the problem metadata using problem service
	problems, err := c.ProblemServiceConfig.GetProblemsByFilters(
		ctx,
		problem_service.GetProblemsRequest{
			PageNumber: 1,
			PageSize:   int32(len(problemIDs)),
			ProblemIDs: problemIDs,
		},
	)
	if err != nil {
		return nil, err
	}

	for _, dbProblem := range dbProblems {
		if problem, ok := problems[dbProblem.ProblemID]; ok {
			res = append(res, ContestProblemResponse{problem, dbProblem.Score})
		} else {
			log.Warnf(
				"contest %v has problem %v registered but missing in fetched filters",
				contest.ID,
				dbProblem.ProblemID,
			)
		}
	}

	return res, nil
}

func (c *ContestService) authorizeProblemView(
	ctx context.Context,
	contest Contest,
) error {
	/*
		1. If the contest is started anyone can see the problems of a contest, otherwise
		2. If its a public contest only authorized personnel can see the problems
		3. If its a private contest, they need the creator access to view the problems

		Since, the creator of a private contest already knows the problems, its ok
		to show them the problems as they also need to edit the problems if needed
	*/

	// contest start time must not be nil
	if contest.StartTime == nil {
		err := fmt.Errorf(
			"%w, contest %v has start time as nil. cannot authorize problem view",
			flux_errors.ErrInternal,
			contest.ID,
		)
		log.Error(err)
		return err
	}

	// contest has started, anyone can view the problems
	if !time.Now().Before(*contest.StartTime) {
		return nil
	}

	// If LockId is set → this contest is public, require lock access authorization.
	if contest.LockId != nil {
		// public contest, they need the get authorized with
		// the access of lock of the contest
		if contest.LockAccess == nil {
			err := fmt.Errorf(
				"%w, lock with id %v associated with contest with id %v has access as nil",
				flux_errors.ErrInternal,
				contest.LockId,
				contest.ID,
			)
			log.Error(err)
			return err
		}

		err := c.UserServiceConfig.AuthorizeUserRole(
			ctx,
			*contest.LockAccess,
			"",
		)
		if err != nil {
			return err
		}
	} else {
		// private contest, need creator access
		err := c.UserServiceConfig.AuthorizeCreatorAccess(
			ctx,
			contest.CreatedBy,
			"",
		)
		if err != nil {
			return err
		}
	}

	return nil
}
