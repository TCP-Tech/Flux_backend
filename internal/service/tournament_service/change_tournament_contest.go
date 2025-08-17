package tournament_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (t *TournamentService) ChangeTournamentContests(
	ctx context.Context,
	request ChangeTournamentContestsRequest,
) ([]contest_service.Contest, error) {
	// fetch claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// authorize (only managers can add contests to a tournament)
	err = t.UserServiceConfig.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf(
			"user %s tried to add a contest to a tournament %v in round %v",
			claims.UserName,
			request.TournamentID,
			request.RoundNumber,
		),
	)
	if err != nil {
		return nil, err
	}

	// fetch tournament by ID to check if it exists
	_, err = t.GetTournamentByID(ctx, request.TournamentID)
	if err != nil {
		return nil, err
	}

	// fetch the latest round
	latestRound, err := t.DB.GetTournamentLatestRound(ctx, request.TournamentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"%w, tournament with ID %v has no rounds",
				flux_errors.ErrNotFound,
				request.TournamentID,
			)
		}
		err = fmt.Errorf(
			"%w, cannot get latest round for tournament %v, %w",
			flux_errors.ErrInternal,
			request.TournamentID,
			err,
		)
		log.Error(err)
		return nil, err
	}
	// check if they are adding contest in latest round
	if latestRound.RoundNumber != request.RoundNumber {
		return nil, fmt.Errorf(
			"%w, cannot add contest to round %v, latest round is %v",
			flux_errors.ErrInvalidRequest,
			request.RoundNumber,
			latestRound.RoundNumber,
		)
	}

	// get all the contests
	contests, err := t.ContestServiceConfig.GetContestsByFilters(
		ctx,
		contest_service.GetContestRequest{
			ContestIDs: request.ContestIDs,
			PageNumber: 1,
			PageSize:   int32(len(request.ContestIDs)),
		},
	)
	if err != nil {
		return nil, err
	}
	// validate the contests
	err = validateTournamentContests(request.ContestIDs, contests)
	if err != nil {
		return nil, err
	}

	// start a transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	// if something goes wrong
	defer tx.Rollback(ctx)

	// prepare a new query tool
	qtx := t.DB.WithTx(tx)

	// delete previous contests
	err = qtx.DeleteTournamentContests(ctx, latestRound.ID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot delete previous contests for tournament round with id %v, %w",
			flux_errors.ErrInternal,
			latestRound.ID,
			err,
		)
		log.Error(err)
		return nil, err
	}

	// add contests to the tournament round
	for _, contest := range contests {
		err = qtx.AddTournamentContest(
			ctx,
			database.AddTournamentContestParams{
				RoundID:   latestRound.ID,
				ContestID: contest.ID,
			},
		)
		if err != nil {
			err = fmt.Errorf(
				"%w, cannot add contest with id %v to tournament round with id %v, %w",
				flux_errors.ErrInternal,
				contest.ID,
				latestRound.ID,
				err,
			)
			log.Error(err)
			return nil, err
		}
	}

	// commit the transaction
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction after adding contests, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return nil, err
	}

	return contests, nil
}

func validateTournamentContests(
	requestContestIDs []uuid.UUID,
	recievedContests []contest_service.Contest,
) error {
	// check the expiry of each contest
	for _, contest := range recievedContests {
		// contest must have a timer lock
		if contest.LockId == nil || contest.LockTimeout == nil {
			return fmt.Errorf(
				"%w, contest with id %v is not associated with a timer lock",
				flux_errors.ErrInvalidRequest,
				contest.ID,
			)
		}

		// 1. contest's start time must not be nil
		// 2. even though its the same as lock timeout its good practice
		//    to use the start time as provided by the contest service by default
		if contest.StartTime == nil {
			err := fmt.Errorf(
				"%w, contest with id %v has timeout as nil when fetched with filters",
				flux_errors.ErrInternal,
				contest.ID,
			)
			log.Error(err)
			return err
		}

		// contest shouldn't have started
		if time.Now().After(*contest.StartTime) {
			return fmt.Errorf(
				"%w, contest with %v has already started",
				flux_errors.ErrInvalidRequest,
				contest.ID,
			)
		}

		// contest shouldn't have published
		if contest.IsPublished {
			return fmt.Errorf(
				"%w, cannot add a published contest to the tournament",
				flux_errors.ErrInvalidRequest,
			)
		}
	}

	if len(recievedContests) > len(requestContestIDs) {
		err := fmt.Errorf(
			"%w, got more contests than requested",
			flux_errors.ErrInternal,
		)
		log.WithField("requested_ids", requestContestIDs).Error(err)
		return err
	}

	if len(recievedContests) < len(requestContestIDs) {
		return fmt.Errorf(
			"%w, some of the contest ids are invalid",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}
