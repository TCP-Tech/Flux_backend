package tournament_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (t *TournamentService) GetTournamentByID(
	ctx context.Context,
	tournamentID uuid.UUID,
) (Tournament, error) {
	// fetch tournament from db
	dbTournament, err := t.DB.GetTournamentById(ctx, tournamentID)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot get torunament with id %v from db", tournamentID),
		)
		return Tournament{}, err
	}

	// convert and return
	return Tournament{
		ID:          dbTournament.ID,
		Title:       dbTournament.Title,
		CreatedBy:   dbTournament.CreatedBy,
		IsPublished: dbTournament.IsPublished,
		Rounds:      dbTournament.Rounds,
	}, nil
}

func (t *TournamentService) GetTournamentByFitlers(
	ctx context.Context,
	request GetTournamentRequest,
) ([]Tournament, error) {
	// validate the request
	err := service.ValidateInput(request)
	if err != nil {
		return nil, err
	}

	offset := (request.PageNumber - 1) * request.PageSize

	dbTournaments, err := t.DB.GetTournamentsByFilters(
		ctx,
		database.GetTournamentsByFiltersParams{
			TitleSearch: request.Title,
			IsPublished: request.IsPublished,
			Limit:       request.PageSize,
			Offset:      offset,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch tournament by filters, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.WithField("request", request).Error(err)
		return nil, err
	}

	res := make([]Tournament, 0, len(dbTournaments))
	for _, dbTournament := range dbTournaments {
		tour := Tournament{
			Title:       dbTournament.Title,
			CreatedBy:   dbTournament.CreatedBy,
			ID:          dbTournament.ID,
			IsPublished: dbTournament.IsPublished,
			Rounds:      dbTournament.Rounds,
		}
		res = append(res, tour)
	}

	return res, nil
}
