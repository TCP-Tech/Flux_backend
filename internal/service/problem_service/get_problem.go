package problem_service

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
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (p *ProblemService) GetProblemById(
	ctx context.Context,
	id int32,
) (Problem, error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	// get the problem from db
	dbProblem, err := p.DB.GetProblemById(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Problem{}, fmt.Errorf(
				"%w, no problem exist with the given id",
				flux_errors.ErrNotFound,
			)
		}
		log.Error(err)
		return Problem{}, fmt.Errorf(
			"%w, cannot fetch problem with id %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
	}

	// authorize
	if dbProblem.LockAccess != nil {
		err = p.LockServiceConfig.AuthorizeLock(
			ctx,
			dbProblem.LockTimeout,
			user_service.UserRole(*dbProblem.LockAccess),
			fmt.Sprintf(
				"user %s tried to access unauthorized problem with id %v",
				claims.UserName,
				id,
			),
		)
		if err != nil {
			if errors.Is(err, flux_errors.ErrUnAuthorized) {
				err = fmt.Errorf(
					"%w, problem with id %v not found",
					flux_errors.ErrNotFound,
					id,
				)
			}
			return Problem{}, err
		}
	}

	// convert to service problem
	serviceProbData, err := getServiceProblemData(
		dbProblem.ExampleTestcases,
		dbProblem.Platform,
	)
	if err != nil {
		return Problem{}, err
	}

	return Problem{
		ID:             dbProblem.ID,
		Title:          dbProblem.Title,
		Statement:      dbProblem.Statement,
		InputFormat:    dbProblem.InputFormat,
		OutputFormat:   dbProblem.OutputFormat,
		Notes:          dbProblem.Notes,
		MemoryLimitKb:  dbProblem.MemoryLimitKb,
		TimeLimitMs:    dbProblem.TimeLimitMs,
		Difficulty:     dbProblem.Difficulty,
		SubmissionLink: dbProblem.SubmissionLink,
		CreatedBy:      dbProblem.CreatedBy,
		LastUpdatedBy:  dbProblem.LastUpdatedBy,
		ExampleTCs:     serviceProbData.exampleTestCases,
		Platform:       serviceProbData.platformType,
		LockId:         dbProblem.LockID,
	}, nil
}

func (p *ProblemService) GetProblemsByFilters(
	ctx context.Context,
	request GetProblemsRequest,
) (map[int32]ProblemMetaData, error) {
	// validate
	valErr := service.ValidateInput(request)
	if valErr != nil {
		return nil, valErr
	}

	// calculate offset
	offset := (request.PageNumber - 1) * request.PageSize

	// get creator
	var createdBy *uuid.UUID
	if request.CreatorRollNo != "" || request.CreatorUserName != "" {
		user, err := p.UserServiceConfig.GetUserByUserNameOrRollNo(
			ctx,
			request.CreatorUserName,
			request.CreatorRollNo,
		)
		if err != nil {
			return nil, err
		}
		createdBy = &user.ID
	}

	// fetch problems from db
	rows, fetchErr := p.DB.GetProblemsByFilters(
		ctx, database.GetProblemsByFiltersParams{
			TitleSearch: request.Title,
			ProblemIds:  request.ProblemIDs,
			LockID:      request.LockID,
			Offset:      offset,
			Limit:       request.PageSize,
			CreatedBy:   createdBy,
		})
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
	res := make(map[int32]ProblemMetaData)
	for _, row := range rows {
		// authorize user for the problem
		// only a handful of people are assigned roles and
		// once fetched they are stored in cache, so better loop and authorize
		// instead of filtering them in the complex sql query
		var lockAccess *user_service.UserRole
		if row.LockAccess != nil {
			err := p.LockServiceConfig.AuthorizeLock(
				ctx,
				row.LockTimeout,
				user_service.UserRole(*row.LockAccess),
				"",
			)
			if err != nil {
				continue
			}
			la := user_service.UserRole(*row.LockAccess)
			lockAccess = &la
		}

		// convert platform
		var platform *Platform
		if row.Platform.Valid {
			plt := Platform(row.Platform.Platform)
			platform = &plt
		}

		// append problem
		pmd := ProblemMetaData{
			ProblemId:   row.ID,
			Title:       row.Title,
			Difficulty:  row.Difficulty,
			Platform:    platform,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt,
			LockID:      row.LockID,
			LockAccess:  lockAccess,
			LockTimeout: row.LockTimeout,
		}
		res[row.ID] = pmd
	}

	return res, nil
}
