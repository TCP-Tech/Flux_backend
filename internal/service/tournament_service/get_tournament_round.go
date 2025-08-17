package tournament_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (t *TournamentService) GetTournamentRound(
	ctx context.Context,
	tournamentID uuid.UUID,
	roundNumber int32,
) (TournamentRound, []contest_service.Contest, error) {
	// get the round
	round, err := t.DB.GetTournamentRoundByNumber(
		ctx,
		database.GetTournamentRoundByNumberParams{
			TournamentID: tournamentID,
			RoundNumber:  roundNumber,
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TournamentRound{}, nil, fmt.Errorf(
				"%w, invalid tournament id or round number",
				flux_errors.ErrInvalidRequest,
			)
		}
		// unknown error
		err = fmt.Errorf(
			"%w, cannot fetch round %v of tournament %v, %w",
			flux_errors.ErrInternal,
			roundNumber,
			tournamentID,
			err,
		)
		log.Error(err)
		return TournamentRound{}, nil, err
	}

	var access user_service.UserRole
	if round.Access != nil {
		access = user_service.UserRole(*round.Access)
	}

	// create a service TournamentRound
	serviceTournamentRound := TournamentRound{
		ID:           round.ID,
		TournamentID: round.TournamentID,
		Title:        round.Title,
		RoundNumber:  round.RoundNumber,
		LockID:       round.LockID,
		CreatedBy:    round.CreatedBy,
		LockAccess:   &access,
		LockTimeout:  round.Timeout,
	}

	// authourize if it has a lock
	if serviceTournamentRound.LockID != nil {
		// access cannot be nil
		if serviceTournamentRound.LockAccess == nil {
			err = fmt.Errorf(
				"%w, tournament round with id %v has non-nil lock but access as nil",
				flux_errors.ErrInternal,
				round.ID,
			)
			log.Error(err)
			return TournamentRound{}, nil, err
		}

		// authourize
		if err = t.LockServiceConfig.AuthorizeLock(
			ctx,
			serviceTournamentRound.LockTimeout,
			*serviceTournamentRound.LockAccess,
			"",
		); err != nil {
			return serviceTournamentRound, nil, nil
		}
	}

	// fetch contests
	contestIDs, err := t.DB.GetTournamentContests(ctx, round.ID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch contests of round %v of tournament %v, %w",
			flux_errors.ErrInternal,
			roundNumber,
			tournamentID,
			err,
		)
		log.Error(err)
		return TournamentRound{}, nil, err
	}

	// get contests using filters
	contests, err := t.ContestServiceConfig.GetContestsByFilters(
		ctx,
		contest_service.GetContestRequest{
			ContestIDs: contestIDs,
			PageNumber: 1,
			PageSize:   int32(len(contestIDs)),
		},
	)
	if err != nil {
		return TournamentRound{}, nil, err
	}

	// handle mismatch bw contestIDs and contests fetched using filters
	if len(contestIDs) != len(contests) {
		log.WithField(
			"requestedIDs",
			contestIDs,
		).Warnf(
			"request %v contests but got %v contests",
			len(contestIDs),
			len(contests),
		)
	}

	return serviceTournamentRound, contests, err
}
