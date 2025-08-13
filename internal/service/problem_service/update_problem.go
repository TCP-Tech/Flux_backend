package problem_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
)

func (p *ProblemService) UpdateProblem(
	ctx context.Context,
	problem Problem,
) (Problem, error) {
	// fetch old problem
	oldProblem, err := p.GetProblemById(
		ctx,
		problem.ID,
	)
	if err != nil {
		return Problem{}, err
	}

	// fetch claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	// authorize
	authErr := p.UserServiceConfig.AuthorizeCreatorAccess(
		ctx,
		oldProblem.CreatedBy,
		fmt.Sprintf(
			"user %s tried to update the problem with id %v",
			claims.UserName,
			problem.ID,
		),
	)
	// user able to see the problem but
	// cannot update the problem, so return authErr
	if authErr != nil {
		return Problem{}, authErr
	}

	// validate the new problem
	valErr := p.validateProblem(ctx, problem)
	if valErr != nil {
		return Problem{}, valErr
	}

	// fetch old lock
	var oldLock *lock_service.FluxLock
	if oldProblem.LockId != nil {
		lock, err := p.LockServiceConfig.GetLockById(
			ctx,
			*oldProblem.LockId,
		)
		if err != nil {
			return Problem{}, err
		}
		oldLock = &lock
	}

	// fetch new lock
	var newLock *lock_service.FluxLock
	if problem.LockId != nil {
		lock, err := p.LockServiceConfig.GetLockById(
			ctx,
			*problem.LockId,
		)
		if err != nil {
			return Problem{}, err
		}
		newLock = &lock
	}

	// validate lock change
	err = p.validateProblemLockChange(oldLock, newLock)
	if err != nil {
		return Problem{}, err
	}

	// update the problem
	params, err := getUpdateProblemParams(claims.UserId, problem)
	if err != nil {
		return Problem{}, err
	}
	updatedProblem, updateErr := p.DB.UpdateProblem(
		ctx,
		params,
	)
	if updateErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(updateErr, &pgErr) {
			if pgErr.Code == flux_errors.CodeUniqueConstraintViolation {
				return Problem{}, fmt.Errorf(
					"%w, %s problem with that key already exist",
					flux_errors.ErrInvalidRequest,
					pgErr.Detail,
				)
			}
		}
		err = fmt.Errorf(
			"%w, unable to insert problem into database, %w",
			flux_errors.ErrInternal,
			updateErr,
		)
		log.Error(err)
		return Problem{}, err
	}

	problem, err = dbProblemToServiceProblem(updatedProblem)

	return problem, err
}

func getUpdateProblemParams(
	updatingUserId uuid.UUID,
	problem Problem,
) (database.UpdateProblemParams, error) {
	dbProblemData, err := getDBProblemDataFromProblem(problem)
	if err != nil {
		return database.UpdateProblemParams{}, err
	}

	// Map fields to AddProblemParams
	return database.UpdateProblemParams{
		ID:               problem.ID,
		Title:            problem.Title,
		Statement:        problem.Statement,
		InputFormat:      problem.InputFormat,
		OutputFormat:     problem.OutputFormat,
		ExampleTestcases: dbProblemData.exampleTestCases, // Note: Typo in field name, should be ExampleTestcases if that's the intended field
		Notes:            problem.Notes,
		MemoryLimitKb:    problem.MemoryLimitKb,
		TimeLimitMs:      problem.TimeLimitMs,
		Difficulty:       problem.Difficulty,
		SubmissionLink:   problem.SubmissionLink,
		Platform:         dbProblemData.platformType,
		LockID:           problem.LockId,
		LastUpdatedBy:    updatingUserId,
	}, nil
}

func (p *ProblemService) validateProblemLockChange(
	oldLock *lock_service.FluxLock,
	newLock *lock_service.FluxLock,
) error {
	// lock is nil
	if oldLock == nil && newLock == nil {
		return nil
	}

	// locking the problem
	if oldLock == nil && newLock != nil {
		// can only assign a manual lock
		if newLock.Type == database.LockTypeTimer {
			return fmt.Errorf(
				"%w, cannot lock the problem with a timer lock once created",
				flux_errors.ErrInvalidRequest,
			)
		}

		return nil
	}

	// unlock the problem
	if oldLock != nil && newLock == nil {
		// can only remove a manual lock
		if oldLock.Type == database.LockTypeTimer {
			return fmt.Errorf(
				"%w, cannot remove timer lock once assigned",
				flux_errors.ErrInvalidRequest,
			)
		}

		return nil
	}

	if oldLock.ID == newLock.ID {
		return nil
	}

	// lock is being changed

	// can only change manual lock
	if oldLock.Type == database.LockTypeTimer || newLock.Type == database.LockTypeTimer {
		return fmt.Errorf(
			"%w, cannot change lock if either old or new lock is a timer lock",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}
