package problem_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/sqlc-dev/pqtype"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (p *ProblemService) validateProblem(
	ctx context.Context,
	problem Problem,
) error {
	// perform validation using validator first
	err := service.ValidateInput(problem)
	if err != nil {
		return err
	}

	// -- extra validations --

	// validate examples
	if problem.ExampleTCs != nil {
		if problem.ExampleTCs.NumTestCases != nil {
			if *problem.ExampleTCs.NumTestCases != len(problem.ExampleTCs.Examples) {
				return fmt.Errorf("%w, num_test_cases != number of example test cases provided", flux_errors.ErrInvalidInput)
			}
		} else if len(problem.ExampleTCs.Examples) > 1 {
			return fmt.Errorf("%w, num_test_cases is nil but number of example test cases are plural", flux_errors.ErrInvalidInput)
		}
	}

	// validate platform
	if problem.Platform != nil {
		if problem.SubmissionLink == nil {
			return fmt.Errorf("%w, platform is provided but submission link is not provided", flux_errors.ErrInvalidInput)
		}
		_, err = p.DB.CheckPlatformType(ctx, *problem.Platform)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) {
				// code for invalid input value
				if pqErr.Code == "22P02" {
					log.Error(pqErr)
					return fmt.Errorf("%w, invalid platform type provided", flux_errors.ErrInvalidInput)
				}
			}
			// Handle any other database errors (e.g., connection failure)
			log.Error("%w, unable to cast platform type", err)
			return fmt.Errorf("%w, unable to cast platform type, %w", flux_errors.ErrInternal, err)
		}
	} else if problem.SubmissionLink != nil {
		return fmt.Errorf("%w, submission link is provided but platform is not provided", flux_errors.ErrInvalidInput)
	}

	return nil
}

// getDatabaseProblemParams prepares the parameters for adding a problem to the database.
// It marshals example test cases, handles nullable fields, and maps the Problem struct to AddProblemParams.
func getAddProblemParams(userId uuid.UUID, problem Problem) (database.AddProblemParams, error) {
	// Marshal example test cases if present
	var exampleTestCases pqtype.NullRawMessage
	if problem.ExampleTCs != nil {
		data, err := json.Marshal(problem.ExampleTCs)
		if err != nil {
			err = fmt.Errorf("%w, unable to marshal %v, %w", flux_errors.ErrInternal, problem.ExampleTCs, err)
			log.Error(err)
			return database.AddProblemParams{}, err
		}
		exampleTestCases.RawMessage = json.RawMessage(data)
		exampleTestCases.Valid = true
	}

	// Prepare notes as a nullable string
	var notes sql.NullString
	if problem.Notes != nil {
		notes.Valid = true
		notes.String = *problem.Notes
	}

	// Prepare submission link as a nullable string
	var submissionLink sql.NullString
	if problem.SubmissionLink != nil {
		submissionLink.Valid = true
		submissionLink.String = *problem.SubmissionLink
	}

	// Prepare platform as a nullable platform type
	var platform database.NullPlatformType
	if problem.Platform != nil {
		platform.Valid = true
		platform.PlatformType = database.PlatformType(*problem.Platform)
	}

	// Map fields to AddProblemParams
	return database.AddProblemParams{
		Title:           problem.Title,
		Statement:       problem.Statement,
		InputFormat:     problem.InputFormat,
		OutputFormat:    problem.OutputFormat,
		EampleTestcases: exampleTestCases, // Note: Typo in field name, should be ExampleTestcases if that's the intended field
		Notes:           notes,
		MemoryLimitKb:   problem.MemoryLimitKB,
		TimeLimitMs:     problem.TimeLimitMS,
		CreatedBy:       userId,
		Difficulty:      problem.Difficulty,
		SubmissionLink:  submissionLink,
		Platform:        platform,
	}, nil
}