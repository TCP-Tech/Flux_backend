package problem_service

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	auth_service "github.com/tcp_snm/flux/internal/service/auth"
	"github.com/tcp_snm/flux/middleware"
)

func (p *ProblemService) UpdateProblem(ctx context.Context, id int32, problem Problem) (err error) {
	// get the user details from claims
	claimsValue := ctx.Value(middleware.KeyCtxUserCredClaims)
	claims, ok := claimsValue.(auth_service.UserCredentialClaims)
	if !ok {
		err = fmt.Errorf(
			"%w, unable to parse claims to auth_service.UserCredentialClaims, type of claims found is %T",
			flux_errors.ErrInternal,
			reflect.TypeOf(claims),
		)
		return
	}

	// fetch user from db
	user, err := p.UserConfig.FetchUserFromDb(ctx, claims.UserName, claims.RollNo)
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
	
	// Fetch the existing problem to check for ownership
	_, err = p.DB.GetProblemById(ctx, id)
	if err != nil {
		if errors.Is(err, flux_errors.ErrNotFound) {
			return fmt.Errorf("%w, problem with id %d not found", flux_errors.ErrNotFound, id)
		}
		log.Error(fmt.Errorf("%w, unable to fetch problem with id %d", err, id))
		return fmt.Errorf("%w, unable to fetch problem with id %d", flux_errors.ErrInternal, id)
	}

	// --- End New Logic ---

	// validate the problem data
	err = p.validateProblem(ctx, problem)
	if err != nil {
		return
	}

	// get UpdateProblemParams (assuming a helper method exists for this)
	params, err := getUpdateProblemParams(id, problem)
	if err != nil {
		return
	}

	// update the problem in the database
	_, err = p.DB.UpdateProblem(ctx, params)
	if err != nil {
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
		log.Errorf("unable to update problem with id %d: %v", id, err)
		err = fmt.Errorf("%w, unable to update problem with id %d: %w", flux_errors.ErrInternal, id, err)
		return
	}

	log.Infof("problem with id %d updated successfully", id)
	return nil
}