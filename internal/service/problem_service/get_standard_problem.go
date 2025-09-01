package problem_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) GetStandardProblemByID(
	ctx context.Context,
	id int32,
) (Problem, StandardProblemData, error) {
	// get the problem
	authProblem, err := p.GetProblemByID(ctx, id)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// get the standard problem data
	dbSpd, err := p.DB.GetStandardProblemData(ctx, id)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot fetch standard_problem_data with id %v", id),
		)
		return Problem{}, StandardProblemData{}, err
	}

	// convert db data to service struct
	spd, err := dbSpdToServiceSpd(dbSpd)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// convert authProblem to service Problem
	problem := Problem{
		ID:         authProblem.ID,
		Title:      authProblem.Title,
		Difficulty: authProblem.Difficulty,
		Evaluator:  authProblem.Evaluator,
		LockID:     authProblem.LockID,
		CreatedBy:  authProblem.CreatedBy,
	}

	return problem, spd, nil
}

func (p *ProblemService) GetProblemsByFilters(
	ctx context.Context,
	request GetProblemsRequest,
) (map[int32]Problem, error) {
	// validate
	valErr := service.ValidateInput(request)
	if valErr != nil {
		return nil, valErr
	}

	// calculate offset
	offset := (request.PageNumber - 1) * request.PageSize

	// get creator
	var createdBy *uuid.UUID
	if request.CreatorUserName != nil || request.CreatorRollNo != nil {
		user, err := p.UserServiceConfig.GetUserByUserNameOrRollNo(
			ctx, request.CreatorUserName,
			request.CreatorRollNo,
		)
		if err != nil {
			return nil, err
		}
		createdBy = &user.UserID
	}

	// fetch problems from db
	dbProblems, fetchErr := p.DB.GetProblemsByFilters(
		ctx, database.GetProblemsByFiltersParams{
			ProblemIds:  request.ProblemIDs,
			LockID:      request.LockID,
			CreatedBy:   createdBy,
			Evaluator:   request.Evaluator,
			TitleSearch: request.Title,
			Offset:      offset,
			Limit:       request.PageSize,
		},
	)
	if fetchErr != nil {
		err := fmt.Errorf(
			"%w, cannot fetch problems with filters from db, %w",
			flux_errors.ErrInternal,
			fetchErr,
		)
		log.WithField("filters", request).Error(err)
		return nil, err
	}

	// convert to meta data
	res := make(map[int32]Problem)
	for _, dbProblem := range dbProblems {
		if dbProblem.Access != nil {
			err := p.LockServiceConfig.AuthorizeLock(
				ctx,
				dbProblem.Timeout,
				*dbProblem.Access,
				"",
			)
			if err != nil {
				continue
			}
		}

		// put problem
		pmd := Problem{
			ID:          dbProblem.ID,
			Title:       dbProblem.Title,
			Difficulty:  dbProblem.Difficulty,
			Evaluator:   dbProblem.Evaluator,
			LockID:      dbProblem.LockID,
			CreatedBy:   dbProblem.CreatedBy,
			LockTimeout: dbProblem.Timeout,
			LockAccess:  dbProblem.Access,
		}
		res[dbProblem.ID] = pmd
	}

	return res, nil
}
