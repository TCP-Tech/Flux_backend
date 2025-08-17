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
	"github.com/tcp_snm/flux/internal/service"
)

func (t *TournamentService) GetTournamentByID(
	ctx context.Context,
	tournamentID uuid.UUID,
) (Tournament, error) {
	// fetch tournament from db
	dbTournament, err := t.DB.GetTournamentById(ctx, tournamentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tournament{}, fmt.Errorf(
				"%w, tournament with ID %v does not exist",
				flux_errors.ErrNotFound,
				tournamentID,
			)
		}
		err = fmt.Errorf(
			"%w, cannot get tournament by ID %v, %w",
			flux_errors.ErrInternal,
			tournamentID,
			err,
		)
		log.Error(err)
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
