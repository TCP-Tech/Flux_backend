package tournament_service

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (t *TournamentService) CreateTournament(
	ctx context.Context,
	tournament Tournament,
) (Tournament, error) {
	// fetch claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Tournament{}, err
	}

	// authorize (only managers can create a tournament)
	err = t.UserServiceConfig.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf("user %s tried to create a tournament", claims.UserName),
	)
	if err != nil {
		return Tournament{}, err
	}

	// validate tournament
	err = service.ValidateInput(tournament)
	if err != nil {
		return Tournament{}, err
	}

	// create tournament
	dbTour, err := t.DB.CreateTournament(ctx, database.CreateTournamentParams{
		Title:       tournament.Title,
		CreatedBy:   claims.UserId,
		IsPublished: tournament.IsPublished,
	})
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot create tournament, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return Tournament{}, err
	}

	// convert and return
	return Tournament{
		Title:       dbTour.Title,
		CreatedBy:   dbTour.CreatedBy,
		ID:          dbTour.ID,
		IsPublished: dbTour.IsPublished,
		Rounds:      0,
	}, nil
}
