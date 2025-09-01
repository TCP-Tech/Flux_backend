package problem_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) GetProblemByID(
	ctx context.Context,
	id int32,
) (Problem, error) {
	// get the problem from db
	dbProblem, err := p.DB.GetProblemByID(ctx, id)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot fetch problem with id %v from db", id),
		)
		return Problem{}, err
	}

	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	// authorize
	if dbProblem.Access != nil {
		err = p.LockServiceConfig.AuthorizeLock(
			ctx,
			dbProblem.Timeout,
			*dbProblem.Access,
			fmt.Sprintf(
				"user %s tried to view unauthorized problem with id %v",
				claims.UserName,
				id,
			),
		)
		if errors.Is(err, flux_errors.ErrUnAuthorized) {
			err = fmt.Errorf(
				"%w, problem with id %v doesn't exist",
				flux_errors.ErrNotFound,
				id,
			)
		}
		return Problem{}, err
	}

	// convert and return
	return Problem{
		ID:          dbProblem.ID,
		Title:       dbProblem.Title,
		Difficulty:  dbProblem.Difficulty,
		Evaluator:   dbProblem.Evaluator,
		LockID:      dbProblem.LockID,
		CreatedBy:   dbProblem.CreatedBy,
		LockTimeout: dbProblem.Timeout,
		LockAccess:  dbProblem.Access,
	}, nil
}
