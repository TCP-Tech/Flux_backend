package problem_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (p *ProblemService) AddProblem(ctx context.Context, problem Problem) (id int32, err error) {
	// get the user details from claims
	user, err := p.UserServiceConfig.FetchUserFromClaims(ctx)
	if err != nil {
		return
	}

	// authorize
	err = p.UserServiceConfig.AuthorizeUserRole(ctx, user.ID, user_service.RoleManager)
	if err != nil {
		if errors.Is(err, flux_errors.ErrUnAuthorized) {
			log.Errorf(
				"%v, user %s tried for manager access to add a problem",
				flux_errors.ErrUnAuthorized,
				user.UserName,
			)
		}
		return
	}

	// validate the problem
	err = p.validateProblem(ctx, problem)
	if err != nil {
		return
	}

	// get AddProblemParams
	params, err := getAddProblemParams(user.ID, problem)
	if err != nil {
		return
	}

	// insert the problem into db
	dbProblem, err := p.DB.AddProblem(ctx, params)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == flux_errors.CodeUniqueConstraintViolation {
				err = fmt.Errorf("%w, %s problem with that key already exist", flux_errors.ErrInvalidInput, pqErr.Detail)
				return
			}
		}
		err = fmt.Errorf("%w, unable to insert problem into database, %w", flux_errors.ErrInternal, err)
		log.Error(err)
		return
	}

	id = dbProblem.ID
	log.Infof("problem with id %d added successfully by user %v", id, user.UserName)
	return
}
