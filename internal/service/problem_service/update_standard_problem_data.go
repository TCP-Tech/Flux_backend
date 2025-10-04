package problem_service

import (
	"context"
	"fmt"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) UpdateStandardProblem(
	ctx context.Context,
	spd StandardProblemData,
) (StandardProblemData, error) {
	// get problem
	problem, err := p.GetProblemByID(ctx, spd.ProblemID)
	if err != nil {
		return StandardProblemData{}, err
	}

	// fetch claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return StandardProblemData{}, err
	}

	// authorize
	err = p.UserServiceConfig.AuthorizeCreatorAccess(
		ctx,
		problem.CreatedBy,
		fmt.Sprintf(
			"user %s tried to update standard_problem with id %v",
			claims.UserName,
			problem.ID,
		),
	)
	if err != nil {
		return StandardProblemData{}, err
	}

	// validate
	err = service.ValidateInput(spd)
	if err != nil {
		return StandardProblemData{}, err
	}

	// updates spd
	updateParams, err := getDbSpdParams(
		spd.FunctionDefinitions,
		spd.ExampleTestCases,
	)
	dbSpd, err := p.DB.UpdateStandardProblemData(
		ctx,
		database.UpdateStandardProblemDataParams{
			ProblemID:          spd.ProblemID,
			Statement:          spd.Statement,
			InputFormat:        spd.InputFormat,
			OutputFormat:       spd.OutputFormat,
			FunctionDefinitons: updateParams.FunctionDefinitions,
			ExampleTestcases:   updateParams.ExampleTestCases,
			Notes:              spd.Notes,
			MemoryLimitKb:      spd.MemoryLimitKB,
			TimeLimitMs:        spd.TimeLimitMS,
			SiteProblemCode:    spd.SiteProblemCode,
			LastUpdatedBy:      claims.UserId,
		},
	)

	// convert dbSpd to serviceSpd
	spdResponse, err := dbSpdToServiceSpd(dbSpd)
	if err != nil {
		return StandardProblemData{}, err
	}

	return spdResponse, nil
}
