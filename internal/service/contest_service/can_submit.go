package contest_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (c *ContestService) CanSubmit(
	ctx context.Context,
	problemID int32,
	contestID uuid.UUID,
) error {
	// get claims from contest
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// get the contest
	contest, err := c.GetContestByID(ctx, contestID)
	if err != nil {
		return err
	}

	// check if the contest has been started
	if contest.StartTime == nil {
		err = fmt.Errorf(
			"%w, contest with id %v queried from GetContestByID func has start time as nil",
			flux_errors.ErrInternal,
			contestID,
		)
		log.Error(err)
		return err
	}
	if time.Now().Before(*contest.StartTime) {
		logrus.Warnf(
			"user %s tried to submit solution to problem with id %v to contest with id %v before start",
			claims.UserName,
			problemID,
			contestID,
		)
		return flux_errors.ErrUnAuthorized
	}

	// contest must be ongoing
	if time.Now().After(contest.EndTime) {
		log.Warnf(
			"user %s tried to submit solution to a completed contest with id %v",
			claims.UserName,
			contestID,
		)
		err = fmt.Errorf(
			"%w, cannot submit to a completed contest",
			flux_errors.ErrInvalidRequest,
		)
		return err
	}

	// check if the problem is in the contest
	has := false
	problems, err := c.GetContestProblems(ctx, contestID)
	if err != nil {
		return err
	}
	for _, problem := range problems {
		if problemID == problem.ProblemData.ID {
			has = true
			break
		}
	}
	if !has {
		log.Warnf(
			"user %s tried to submit solution to problem with id %v that doesn't exist in contest with id %v",
			claims.UserName,
			problemID,
			contestID,
		)
		return fmt.Errorf(
			"%w, problem doesn't exist in the given contest",
			flux_errors.ErrInvalidRequest,
		)
	}

	// check if the contest is public and published
	if contest.LockId != nil && contest.IsPublished {
		return nil
	}

	// check if the user is registered to the contest
	registered := false
	registeredUsers, err := c.GetContestRegisteredUsers(ctx, contestID)
	if err != nil {
		return err
	}
	for _, user := range registeredUsers {
		if claims.UserId == user.UserID {
			registered = true
			break
		}
	}
	if !registered {
		log.Warnf(
			"unregistered user %s tried to submit solution to problem with id %v to contest with id %v",
			claims.UserName,
			problemID,
			contestID,
		)
		return flux_errors.ErrUnAuthorized
	}

	return nil
}