package problem_service

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (p *ProblemService) AddStandardProblem(
	ctx context.Context,
	problemRequest Problem,
	spdRequest StandardProblemData,
) (Problem, StandardProblemData, error) {
	// get the user details from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
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
		return Problem{}, StandardProblemData{}, err
	}

	// validate problem
	if problemRequest.LockID != nil {
		lock, err := p.LockServiceConfig.GetLockById(ctx, *problemRequest.LockID)
		if err != nil {
			return Problem{}, StandardProblemData{}, err
		}
		problemRequest.LockTimeout = lock.Timeout
		problemRequest.LockAccess = &lock.Access
	}
	err = p.validateProblem(problemRequest)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// validate the data
	if err = p.validateStandardProblemData(spdRequest); err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// create a transaction
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// crate a query tool with tx
	qtx := p.DB.WithTx(tx)
	defer tx.Rollback(ctx)

	// insert the problem first
	problemResponse, err := insertProblemToDB(ctx, qtx, problemRequest)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// insert the spd
	spdParams, err := getDbSpdParams(
		spdRequest.FunctionDefinitions,
		spdRequest.ExampleTestCases,
	)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}
	dbSPD, err := qtx.AddStandardProblemData(ctx, database.AddStandardProblemDataParams{
		ProblemID:          problemResponse.ID,
		Statement:          spdRequest.Statement,
		InputFormat:        spdRequest.InputFormat,
		OutputFormat:       spdRequest.OutputFormat,
		FunctionDefinitons: spdParams.FunctionDefinitions,
		ExampleTestcases:   spdParams.ExampleTestCases,
		Notes:              spdRequest.Notes,
		MemoryLimitKb:      spdRequest.MemoryLimitKB,
		TimeLimitMs:        spdRequest.TimeLimitMS,
		SubmissionLink:     spdRequest.SubmissionLink,
	})
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			"failed to insert standard_problem_data into db",
		)
		return Problem{}, StandardProblemData{}, err
	}

	// 1. convert to service data
	// 2. even though its the same data as in request, its best practice
	//    to always convert the data from db to service and return
	spdResponse, err := dbSpdToServiceSpd(dbSPD)
	if err != nil {
		return Problem{}, StandardProblemData{}, err
	}

	// commit the tx
	if err = tx.Commit(ctx); err != nil {
		err = fmt.Errorf(
			"%w, cannot commit transaction while adding a standard problem to db, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return Problem{}, StandardProblemData{}, err
	}

	return problemResponse, spdResponse, nil
}
