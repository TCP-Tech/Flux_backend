package problem_service

import (
	"context"
	"fmt"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) UpdateProblem(
	ctx context.Context,
	problem Problem,
) (Problem, error) {
	// get old problem
	oldProblem, err := p.GetProblemByID(ctx, problem.ID)
	if err != nil {
		return Problem{}, err
	}

	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	// authorize
	if err = p.UserServiceConfig.AuthorizeCreatorAccess(
		ctx,
		problem.CreatedBy,
		fmt.Sprintf(
			"user %s tried to update unauthorized problem with id %v",
			claims.UserName,
			problem.ID,
		),
	); err != nil {
		return Problem{}, err
	}

	// validate
	if err = p.validateProblemUpdate(ctx, oldProblem, problem); err != nil {
		return Problem{}, err
	}

	// update
	dbProblem, err := updateProblem(
		ctx, p.DB,
		database.UpdateProblemParams{
			ID:            problem.ID,
			Title:         problem.Title,
			Difficulty:    problem.Difficulty,
			Evaluator:     problem.Evaluator,
			LockID:        problem.LockID,
			LastUpdatedBy: claims.UserId,
		},
	)
	if err != nil {
		return Problem{}, err
	}

	// convert and return
	return Problem{
		ID:         dbProblem.ID,
		Title:      dbProblem.Title,
		Difficulty: dbProblem.Difficulty,
		Evaluator:  dbProblem.Evaluator,
		LockID:     dbProblem.LockID,
		CreatedBy:  dbProblem.CreatedBy,
	}, nil
}

func (p *ProblemService) validateProblemUpdate(
	ctx context.Context,
	oldProblem Problem,
	newProblem Problem,
) error {
	// validate new problem
	err := service.ValidateInput(newProblem)
	if err != nil {
		return err
	}

	// validate lock
	if oldProblem.LockID == nil && newProblem.LockID == nil {
		return nil
	}

	// problem is being unlocked
	if oldProblem.LockID != nil && newProblem.LockID == nil {
		// timer lock cannot be removed
		if oldProblem.LockTimeout != nil {
			return fmt.Errorf(
				"%w, timer locked problem cannot be unlocked",
				flux_errors.ErrInvalidRequest,
			)
		}
		return nil
	}

	// new problem's lock is not nil
	newProblemLock, err := p.LockServiceConfig.GetLockById(ctx, *newProblem.LockID)
	if err != nil {
		return err
	}

	// problem is being locked
	if oldProblem.LockID == nil {
		if newProblemLock.Type == database.LockTypeTimer {
			return fmt.Errorf(
				"%w, cannot lock a problem with timer lock after creation",
				flux_errors.ErrInvalidRequest,
			)
		}
		return nil
	}

	// lock is same
	if oldProblem.LockID == newProblem.LockID {
		return nil
	}

	// lock is being changed
	// timer lock cannot be changed
	if oldProblem.LockTimeout != nil {
		return fmt.Errorf(
			"%w, cannot update lock of a timer locked problem",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}
