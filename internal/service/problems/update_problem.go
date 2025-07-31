package problem_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (p *ProblemService) UpdateProblem(
	ctx context.Context,
	problem Problem,
) (problemResponse Problem, err error) {
	// get the user details from claims
	user, err := p.UserConfig.FetchUserFromClaims(ctx)
	if err != nil {
		return
	}

	// authorize manager access
	err = p.UserConfig.AuthorizeManager(ctx, user.ID)
	if err != nil {
		if errors.Is(err, flux_errors.ErrUnAuthorized) {
			log.Errorf(
				"%v, user %s tried for manager access to update a problem",
				flux_errors.ErrUnAuthorized,
				user.UserName,
			)
		}
		return
	}

	// validate the problem data
	err = p.validateProblem(ctx, problem)
	if err != nil {
		return
	}

	// get UpdateProblemParams
	params, err := getUpdateProblemParams(user.ID, problem.ID, problem)
	if err != nil {
		return
	}

	// update the problem in the database
	dbProblem, err := p.DB.UpdateProblem(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("%w, problem with id %d do not exist", flux_errors.ErrNotFound, problem.ID)
			log.Error(err)
			return
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == flux_errors.CodeUniqueConstraintViolation {
				err = fmt.Errorf(
					"%w, problem with that key already exists",
					flux_errors.ErrInvalidInput,
				)
				return
			}
		}
		log.Errorf("unable to update problem with id %d: %v", problem.ID, err)
		err = fmt.Errorf(
			"%w, unable to update problem with id %d: %w",
			flux_errors.ErrInternal,
			problem.ID, err,
		)
		return
	}
	log.Infof("problem with id %d updated successfully by user %v", problem.ID, user.UserName)

	// prepare the response
	problemResponse, err = dbProblemToServiceProblem(dbProblem)
	return
}
