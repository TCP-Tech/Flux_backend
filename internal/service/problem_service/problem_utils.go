package problem_service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) validateProblem(
	ap Problem,
) error {
	// raw validations
	err := service.ValidateInput(ap)
	if err != nil {
		return err
	}

	// validate its lock
	if ap.LockTimeout != nil {
		if time.Now().After(*ap.LockTimeout) {
			return fmt.Errorf(
				"%w, expired lock cannot be used to create problem",
				flux_errors.ErrInvalidRequest,
			)
		}
	}

	return nil
}

func (p *ProblemService) validateStandardProblemData(spd StandardProblemData) error {
	// perform validation using validator first
	err := service.ValidateInput(spd)
	if err != nil {
		return err
	}

	// -- extra validations --

	// validate examples
	if spd.ExampleTestCases != nil {
		if spd.ExampleTestCases.NumTestCases != nil {
			if *spd.ExampleTestCases.NumTestCases != len(spd.ExampleTestCases.Examples) {
				return fmt.Errorf(
					"%w, num_test_cases != number of example test cases provided",
					flux_errors.ErrInvalidRequest,
				)
			}
		} else if len(spd.ExampleTestCases.Examples) > 1 {
			return fmt.Errorf(
				"%w, num_test_cases is nil but number of example test cases are plural",
				flux_errors.ErrInvalidRequest,
			)
		}
	}

	return nil
}

func getDbSpdParams(
	functionDefinitions map[string]string,
	exampleTestCases *ExampleTestCases,
) (dbSpdParams, error) {
	// marshal function definitions
	fdBytes, err := json.Marshal(functionDefinitions)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			functionDefinitions,
			err,
		)
		logrus.Error(err)
		return dbSpdParams{}, err
	}
	// marshal example testcases
	etcsBytes, err := json.Marshal(exampleTestCases)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			exampleTestCases,
			err,
		)
		logrus.Error(err)
		return dbSpdParams{}, err
	}

	// wrap in json.RawMessage
	fdJson := json.RawMessage(fdBytes)
	etcsJson := json.RawMessage(etcsBytes)

	return dbSpdParams{
		FunctionDefinitions: &fdJson,
		ExampleTestCases:    &etcsJson,
	}, nil
}

func dbSpdToServiceSpd(
	dbSpd database.StandardProblemDatum,
) (StandardProblemData, error) {
	var fd map[string]string
	var etcs *ExampleTestCases

	// unmarshal fd
	if dbSpd.FunctionDefinitons != nil {
		err := json.Unmarshal([]byte(*dbSpd.FunctionDefinitons), &fd)
		if err != nil {
			err = fmt.Errorf(
				"%w, cannot unmarshal %v, %w",
				flux_errors.ErrInternal,
				dbSpd.FunctionDefinitons,
				err,
			)
			log.Error(err)
			return StandardProblemData{}, err
		}
	}

	// unmarshal etcs
	if dbSpd.ExampleTestcases != nil {
		var e ExampleTestCases
		err := json.Unmarshal([]byte(*dbSpd.ExampleTestcases), &e)
		if err != nil {
			err = fmt.Errorf(
				"%w, cannot unmarshal %v, %w",
				flux_errors.ErrInternal,
				dbSpd.ExampleTestcases,
				err,
			)
			log.Error(err)
			return StandardProblemData{}, err
		}
		etcs = &e
	}

	return StandardProblemData{
		ProblemID:           dbSpd.ProblemID,
		Statement:           dbSpd.Statement,
		InputFormat:         dbSpd.InputFormat,
		OutputFormat:        dbSpd.OutputFormat,
		FunctionDefinitions: fd,
		ExampleTestCases:    etcs,
		Notes:               dbSpd.Notes,
		MemoryLimitKB:       dbSpd.MemoryLimitKb,
		TimeLimitMS:         dbSpd.TimeLimitMs,
		SubmissionLink:      dbSpd.SubmissionLink,
	}, nil

}

func insertProblemToDB(
	ctx context.Context,
	qtx *database.Queries,
	problem Problem,
) (Problem, error) {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return Problem{}, err
	}

	dbProblem, err := qtx.AddProblem(ctx, database.AddProblemParams{
		Title:         problem.Title,
		Difficulty:    problem.Difficulty,
		Evaluator:     problem.Evaluator,
		LockID:        problem.LockID,
		CreatedBy:     claims.UserId,
		LastUpdatedBy: claims.UserId,
	})
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			"failed to insert problem into db",
		)
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

// helper function to update problem
// since there can be various types of problems and each type has its own update
// method, this helps in avoiding code duplicacy
func updateProblem(
	ctx context.Context,
	qtx *database.Queries,
	params database.UpdateProblemParams,
) (database.Problem, error) {
	// update problem
	dbProblem, err := qtx.UpdateProblem(ctx, params)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("failed to update problem with id %v", dbProblem.ID),
		)
		return database.Problem{}, err
	}

	return dbProblem, nil
}
