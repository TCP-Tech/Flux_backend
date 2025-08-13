package problem_service

import (
	"context"
	"database/sql"
	"encoding/json"
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
				return fmt.Errorf(
					"%w, num_test_cases != number of example test cases provided",
					flux_errors.ErrInvalidRequest,
				)
			}
		} else if len(problem.ExampleTCs.Examples) > 1 {
			return fmt.Errorf(
				"%w, num_test_cases is nil but number of example test cases are plural",
				flux_errors.ErrInvalidRequest,
			)
		}
	}

	// validate platform
	if problem.Platform != nil {
		if problem.SubmissionLink == nil {
			return fmt.Errorf("%w, platform is provided but submission link is not provided", flux_errors.ErrInvalidRequest)
		}
		_, err = p.DB.CheckPlatformType(ctx, database.Platform(*problem.Platform))
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				// code for invalid input value
				if pgErr.Code == "22P02" {
					log.Error(pgErr)
					return fmt.Errorf("%w, invalid platform type provided", flux_errors.ErrInvalidRequest)
				}
			}
			// Handle any other database errors (e.g., connection failure)
			log.Error("%w, unable to cast platform type", err)
			return fmt.Errorf("%w, unable to cast platform type, %w", flux_errors.ErrInternal, err)
		}
	} else if problem.SubmissionLink != nil {
		return fmt.Errorf("%w, submission link is provided but platform is not provided", flux_errors.ErrInvalidRequest)
	}

	// lock has different validations for different purposes

	return nil
}

// getDBProblemDataFromProblem converts a service Problem struct to a database DBProblemData.
// The nullable fields are correctly prepared here.
func getDBProblemDataFromProblem(problem Problem) (dbProblemData, error) {
	var exampleTestCases *json.RawMessage
	if problem.ExampleTCs != nil {
		bytes, marsErr := json.Marshal(*problem.ExampleTCs)
		if marsErr != nil {
			err := fmt.Errorf(
				"%w, cannot marshal %v, %w",
				flux_errors.ErrInternal,
				problem.ExampleTCs,
				marsErr,
			)
			log.Error(err)
			return dbProblemData{}, err
		}
		rawMessage := json.RawMessage(bytes)
		exampleTestCases = &rawMessage
	}

	var platformType database.NullPlatform
	if problem.Platform != nil {
		platformType.Valid = true
		platformType.Platform = database.Platform(*problem.Platform)
	}

	return dbProblemData{
		exampleTestCases: exampleTestCases,
		platformType:     platformType,
	}, nil
}

func getServiceProblemData(
	exampleTestCasesJson *json.RawMessage,
	dbPlatformType database.NullPlatform,
) (serviceProblemData, error) {
	// unmarshal
	var exampleTestCases *ExampleTestCases
	if exampleTestCasesJson != nil {
		var etcs ExampleTestCases
		marsErr := json.Unmarshal([]byte(*exampleTestCasesJson), &etcs)
		if marsErr != nil {
			err := fmt.Errorf(
				"%w, cannot unamrshal %v, %w",
				flux_errors.ErrInternal,
				exampleTestCasesJson,
				marsErr,
			)
			log.Error(err)
			return serviceProblemData{}, err
		}
		exampleTestCases = &etcs
	}

	var platformType *Platform
	if dbPlatformType.Valid {
		pt := Platform(dbPlatformType.Platform)
		platformType = &pt
	}

	return serviceProblemData{
		exampleTestCases: exampleTestCases,
		platformType:     platformType,
	}, nil
}

func dbProblemToServiceProblem(
	dbProblem database.Problem,
) (Problem, error) {
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

func (p *ProblemService) AuthorizeProblem(
	ctx context.Context,
	id int32,
	warnMessage string,
) (*uuid.UUID, error) {
	// get lockID
	auth, err := p.DB.GetProblemAuth(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf(
				"%w, no problem exist with given id",
				flux_errors.ErrInvalidRequest,
			)
			return nil, err
		}
	}

	// authorize
	if auth.ID != nil {
		if auth.Access == nil {
			err = fmt.Errorf(
				"%w, lock with id %v has access as nil",
				flux_errors.ErrInternal,
				auth.ID,
			)
			log.Error(err)
			return nil, err
		}
		err = p.LockServiceConfig.AuthorizeLock(
			ctx,
			auth.Timeout,
			user_service.UserRole(*auth.Access),
			warnMessage,
		)
		if err != nil {
			return nil, err
		}
	}

	return auth.ID, nil
}
