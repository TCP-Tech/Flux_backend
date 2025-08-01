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

// It marshals example test cases, handles nullable fields, and
// maps the Problem struct to AddProblemParams.
func getDBProblemDataFromProblem(problem Problem) (DBProblemData, error) {
	var exampleTestCases pqtype.NullRawMessage
	if problem.ExampleTCs != nil {
		data, err := json.Marshal(problem.ExampleTCs)
		if err != nil {
			err = fmt.Errorf("%w, unable to marshal %v, %w", flux_errors.ErrInternal, problem.ExampleTCs, err)
			log.Error(err)
			return DBProblemData{}, err
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

	return DBProblemData{
		exampleTestCases: exampleTestCases,
		notes:            notes,
		submissionLink:   submissionLink,
		platform:         platform,
	}, nil
}

// getDatabaseProblemParams prepares the parameters for adding a problem to the database.
func getAddProblemParams(userId uuid.UUID, problem Problem) (database.AddProblemParams, error) {
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
		Notes:            dbProblemData.notes,
		MemoryLimitKb:    problem.MemoryLimitKB,
		TimeLimitMs:      problem.TimeLimitMS,
		CreatedBy:        userId,
		Difficulty:       problem.Difficulty,
		SubmissionLink:   dbProblemData.submissionLink,
		Platform:         dbProblemData.platform,
	}, nil
}

func getUpdateProblemParams(userId uuid.UUID, problemId int32, problem Problem) (database.UpdateProblemParams, error) {
	dbProblemData, err := getDBProblemDataFromProblem(problem)
	if err != nil {
		return database.UpdateProblemParams{}, err
	}

	// Map fields to AddProblemParams
	return database.UpdateProblemParams{
		Title:            problem.Title,
		Statement:        problem.Statement,
		InputFormat:      problem.InputFormat,
		OutputFormat:     problem.OutputFormat,
		ExampleTestcases: dbProblemData.exampleTestCases, // Note: Typo in field name, should be ExampleTestcases if that's the intended field
		Notes:            dbProblemData.notes,
		MemoryLimitKb:    problem.MemoryLimitKB,
		TimeLimitMs:      problem.TimeLimitMS,
		LastUpdatedBy:    userId,
		Difficulty:       problem.Difficulty,
		SubmissionLink:   dbProblemData.submissionLink,
		Platform:         dbProblemData.platform,
		ID:               problemId,
	}, nil
}

func dbProblemToServiceProblem(dbProblem database.Problem) (problem Problem, err error) {
	// extract exampleTestCases
	var exampleTestCases ExampleTestCases
	if dbProblem.ExampleTestcases.Valid {
		// unmarshal the data
		err = json.Unmarshal(
			[]byte(dbProblem.ExampleTestcases.RawMessage),
			&exampleTestCases,
		)
		if err != nil {
			err = fmt.Errorf(
				"%w, unable to unmarshal example testcases of problem with id %d, %w",
				flux_errors.ErrInternal,
				problem.ID,
				err,
			)
			log.Error(err)
			return
		}
	}

	var notes string
	if dbProblem.Notes.Valid {
		notes = dbProblem.Notes.String
	}

	var submissionLink string
	if dbProblem.SubmissionLink.Valid {
		submissionLink = dbProblem.SubmissionLink.String
	}

	var platform string
	if dbProblem.Platform.Valid {
		platform = string(dbProblem.Platform.PlatformType)
	}
	return Problem{
		ID:             dbProblem.ID,
		Title:          dbProblem.Title,
		Statement:      dbProblem.Statement,
		InputFormat:    dbProblem.InputFormat,
		OutputFormat:   dbProblem.OutputFormat,
		ExampleTCs:     &exampleTestCases,
		Notes:          &notes,
		MemoryLimitKB:  dbProblem.MemoryLimitKb,
		TimeLimitMS:    dbProblem.TimeLimitMs,
		Difficulty:     dbProblem.Difficulty,
		SubmissionLink: &submissionLink,
		Platform:       &platform,
		CreatedBy:      dbProblem.CreatedBy,
		LastUpdatedBy:  dbProblem.LastUpdatedBy,
	}, nil
}

func dbProblemsToServiceProblems(dbProblems []database.Problem) ([]Problem, error) {
	problems := make([]Problem, 0, len(dbProblems))
	var convErr error = nil
	for _, dbProblem := range dbProblems {
		problem, e := dbProblemToServiceProblem(dbProblem)
		if e != nil {
			convErr = fmt.Errorf("%w, %w", flux_errors.ErrPartialResult, e)
			continue
		}
		problems = append(problems, problem)
	}
	return problems, convErr
}
