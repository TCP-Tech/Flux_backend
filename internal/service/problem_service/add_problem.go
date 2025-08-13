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
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (p *ProblemService) AddProblem(
	ctx context.Context,
	problem Problem,
) (Problem, error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	// authorize (only managers can add problems)
	err = p.UserServiceConfig.AuthorizeUserRole(
		ctx, user_service.RoleManager,
		fmt.Sprintf(
			"user %s tried for manager access to add a problem",
			claims.UserName,
		),
	)
	if err != nil {
		return Problem{}, err
	}

	// validate the problem
	err = p.validateProblem(ctx, problem)
	if err != nil {
		return Problem{}, err
	}

	// validate lock
	if problem.LockId != nil {
		// get the lock (authorizes by default)
		lock, err := p.LockServiceConfig.GetLockById(
			ctx,
			*problem.LockId,
		)
		if err != nil {
			return Problem{}, err
		}
		
		// validate the expiry
		exp, err := p.LockServiceConfig.IsLockExpired(lock, 5)
		if err != nil {
			return Problem{}, err
		}
		if exp {
			return Problem{}, fmt.Errorf(
				"%w, lock's expiry must be atleast 5 mins from now",
				flux_errors.ErrInvalidRequest,
			)
		}
	}

	// convert service params to db params
	params, err := getAddProblemParams(claims.UserId, problem)
	if err != nil {
		return Problem{}, err
	}

	// insert the problem into db
	dbProblem, err := p.DB.AddProblem(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == flux_errors.CodeUniqueConstraintViolation {
				err = fmt.Errorf(
					"%w, %s problem with that key already exist",
					flux_errors.ErrInvalidRequest,
					pgErr.Detail,
				)
				return Problem{}, err
			}
		}
		err = fmt.Errorf(
			"%w, unable to insert problem into database, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return Problem{}, err
	}

	log.Infof(
		"problem with id %v was created successfully by user %s",
		dbProblem.ID,
		claims.UserName,
	)

	problem, err = dbProblemToServiceProblem(dbProblem)
	return problem, err
}

// getDatabaseProblemParams prepares the parameters for adding a problem to the database.
func getAddProblemParams(
	userId uuid.UUID,
	problem Problem,
) (database.AddProblemParams, error) {
	dbProblemData, err := getDBProblemDataFromProblem(problem)
	if err != nil {
		return database.AddProblemParams{}, err
	}

	// Map fields to AddProblemParams
	return database.AddProblemParams{
		Title:            problem.Title,
		Statement:        problem.Statement,
		InputFormat:      problem.InputFormat,
		OutputFormat:     problem.OutputFormat,
		ExampleTestcases: dbProblemData.exampleTestCases, // Note: Typo in field name, should be ExampleTestcases if that's the intended field
		Notes:            problem.Notes,
		MemoryLimitKb:    problem.MemoryLimitKb,
		TimeLimitMs:      problem.TimeLimitMs,
		CreatedBy:        userId,
		Difficulty:       problem.Difficulty,
		SubmissionLink:   problem.SubmissionLink,
		Platform:         dbProblemData.platformType,
		LockID:           problem.LockId,
	}, nil
}
