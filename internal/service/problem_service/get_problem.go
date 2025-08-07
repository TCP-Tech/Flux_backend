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
		return Problem{}, fmt.Errorf(
			"%w, cannot fetch problem with id %v, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
	}

	// authorize
	if dbProblem.Access != nil {
		access := user_service.UserRole(*dbProblem.Access)
		accessErr := p.UserServiceConfig.AuthorizeUserRole(
			ctx,
			claims.UserId,
			access,
			fmt.Sprintf(
				"user %s tried to access unauthorized problem with id %v",
				claims.UserName,
				id,
			),
		)
		if accessErr != nil {
			return Problem{}, fmt.Errorf(
				"%w, no problem exist with the given id",
				flux_errors.ErrNotFound,
			)
		}
	}

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
		MemoryLimitKB:  dbProblem.MemoryLimitKb,
		TimeLimitMS:    dbProblem.TimeLimitMs,
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
) ([]ProblemMetaData, error) {
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
		user, err := p.UserServiceConfig.FetchUserFromDb(ctx, request.CreatorUserName, request.CreatorRollNo)
		if err != nil {
			return nil, err
		}
		createdBy = &user.ID
	}

	// fetch problems from db
	rows, fetchErr := p.DB.GetProblemsByFilters(
		ctx, database.GetProblemsByFiltersParams{
			Title:     fmt.Sprintf("%%%s%%", request.Title),
			Offset:    offset,
			Limit:     request.PageSize,
			CreatedBy: createdBy,
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
	log.Info(rows)

	// fetch claims to access userID
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// convert to meta data
	problemsMetaData := make([]ProblemMetaData, 0, len(rows))
	for _, row := range rows {
		// authorize user for the problem
		// only a handful of people are assigned roles and
		// once fetched they are stored in cache, so better loop and authorize
		// instead of filtering them in the complex sql query
		if row.LockAccess != nil {
			accessErr := p.UserServiceConfig.AuthorizeUserRole(
				ctx,
				claims.UserId,
				user_service.UserRole(*row.LockAccess),
				"",
			)
			if accessErr != nil {
				continue
			}
		}

		// convert platform
		var platform *Platform
		if row.Platform.Valid {
			plt := Platform(row.Platform.Platform)
			platform = &plt
		}

		// append problem
		pmd := ProblemMetaData{
			ProblemId:  row.ID,
			Title:      row.Title,
			Difficulty: row.Difficulty,
			Platform:   platform,
			CreatedBy:  row.CreatedBy,
			CreatedAt:  row.CreatedAt,
		}
		problemsMetaData = append(problemsMetaData, pmd)
	}

	return problemsMetaData, nil
}
