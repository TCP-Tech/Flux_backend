package tournament_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (t *TournamentService) CreateTournamentRound(
	ctx context.Context,
	tournamentRound TournamentRound,
) (TournamentRound, error) {
	// fetch claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return TournamentRound{}, err
	}

	// authorize (only managers can create a tournament round)
	err = t.UserServiceConfig.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried to create a tournament round", claims.UserName),
	)
	if err != nil {
		return TournamentRound{}, err
	}

	// get the previous tournament
	tournament, err := t.GetTournamentByID(ctx, tournamentRound.TournamentID)
	if err != nil {
		return TournamentRound{}, err
	}

	// check if previous round ended
	endTime, err := t.DB.GetTournamentLatestRoundEndTime(ctx, tournamentRound.TournamentID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot get previous round end time of tournament %v, %w",
			flux_errors.ErrInternal,
			tournamentRound.TournamentID,
			err,
		)
		log.Error(err)
		return TournamentRound{}, err
	}
	if time.Now().Before(endTime.UTC()) {
		return TournamentRound{}, fmt.Errorf(
			"%w, cannot create new round, previous round has not ended yet",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate tournament round
	err = service.ValidateInput(tournamentRound)
	if err != nil {
		return TournamentRound{}, err
	}

	// validate new round's lock
	if tournamentRound.LockID == nil {
		return TournamentRound{}, fmt.Errorf(
			"%w, round must be associated with a lock while creation",
			flux_errors.ErrInvalidRequest,
		)
	}
	err = t.validateTournamentRoundLock(ctx, *tournamentRound.LockID)
	if err != nil {
		return TournamentRound{}, err
	}

	// create tournament round
	dbRound, err := t.DB.CreateTournamentRound(ctx,
		database.CreateTournamentRoundParams{
			TournamentID: tournamentRound.TournamentID,
			LockID:       tournamentRound.LockID,
			Title:        tournamentRound.Title,
			RoundNumber:  tournament.Rounds + 1,
			CreatedBy:    claims.UserId,
		},
	)
	if err != nil {
		var pgErr *pgconn.PgError
		// check if its not a known error from client side
		if !errors.As(err, &pgErr) ||
			pgErr.Code != flux_errors.CodeForeignKeyConstraint {
			err = fmt.Errorf(
				"%w, cannot create tournament round, %w",
				flux_errors.ErrInternal,
				err,
			)
			log.Error(err)
			return TournamentRound{}, err
		}

		// its a foreign key error, check which one
		msg, ok := dbConstraintMessages[pgErr.ConstraintName]
		if !ok {
			msg = pgErr.Detail
			log.Errorf(
				"unknown foreign key error while creating a tournament round: %s",
				pgErr.ConstraintName,
			)
		}
		return TournamentRound{}, fmt.Errorf(
			"%w, %s",
			flux_errors.ErrInvalidRequest,
			msg,
		)
	}

	// return response
	return TournamentRound{
		ID:           dbRound.ID,
		TournamentID: dbRound.TournamentID,
		Title:        dbRound.Title,
		RoundNumber:  dbRound.RoundNumber,
		LockID:       dbRound.LockID,
		CreatedBy:    dbRound.CreatedBy,
	}, nil
}
