package contest_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	log "github.com/sirupsen/logrus"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (c *ContestService) validatePrivateContest(
	contest Contest,
) error {
	// raw validations
	err := service.ValidateInput(contest)
	if err != nil {
		return err
	}

	// lock_id must be nil
	if contest.LockId != nil {
		err = fmt.Errorf(
			"%w, private contests cannot have locks",
			flux_errors.ErrInvalidRequest,
		)
		log.Warn(err)
		return err
	}

	if contest.StartTime == nil {
		return fmt.Errorf(
			"%w, start time must be specified for private contests",
			flux_errors.ErrInvalidRequest,
		)
	}

	// contest must start after 1 minutes from now atleast
	if time.Now().Add(time.Minute * 1).After(*contest.StartTime) {
		return fmt.Errorf(
			"%w, contest must start after atleast 1 minutes from now",
			flux_errors.ErrInvalidRequest,
		)
	}

	// start time must be before end time
	// !Before ensures startTime != endTime
	if !contest.StartTime.Before(contest.EndTime) {
		return fmt.Errorf(
			"%w, start time must be less than end time",
			flux_errors.ErrInvalidRequest,
		)
	}

	// private contest cannot be published
	if contest.IsPublished {
		err = fmt.Errorf(
			"%w, private contests cannot be published",
			flux_errors.ErrInvalidRequest,
		)
		log.Error(err)
		return err
	}

	return nil
}

func (c *ContestService) validatePublicContest(
	request CreateContestRequest,
	lock lock_service.FluxLock,
) error {
	// raw validations
	err := service.ValidateInput(request.ContestDetails)
	if err != nil {
		return err
	}

	// start time is inferred only from its lock
	if request.ContestDetails.StartTime != nil {
		return fmt.Errorf(
			"%w, start time of a public contest must be null, it is inferred from its lock only",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate type
	if lock.Type != database.LockTypeTimer {
		return fmt.Errorf(
			"%w, only timer locks can be used for public contest",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate its expiry
	expired, err := c.LockServiceConfig.IsLockExpired(lock, 60*24)
	if err != nil {
		return err
	}
	if expired {
		return fmt.Errorf(
			"%w, lock must have atleast one day of expiry",
			flux_errors.ErrInvalidRequest,
		)
	}

	// validate end time
	if lock.Timeout.Add(time.Minute * 5).After(request.ContestDetails.EndTime) {
		return fmt.Errorf(
			"%w, contest endtime must be atleast 5 minutes after the expiry of the lock",
			flux_errors.ErrInvalidRequest,
		)
	}

	// published contest cannot have any users registered
	if request.ContestDetails.IsPublished && len(request.RegisteredUsers) > 0 {
		return fmt.Errorf(
			"%w, published contest cannot have any registered users",
			flux_errors.ErrInvalidRequest,
		)
	}

	return nil
}

func (c *ContestService) validateContestProblems(
	ctx context.Context,
	contestLockID *uuid.UUID,
	problems []ContestProblem,
) error {
	// check for empty
	if len(problems) == 0 {
		return nil
	}

	// get their ids into a slice
	problemIDs := make([]int32, 0, len(problems))
	for _, problem := range problems {
		// raw validation
		err := service.ValidateInput(problem)
		if err != nil {
			return err
		}
		problemIDs = append(problemIDs, problem.ProblemId)
	}

	// fetch problems (Get method also perform auth implicitly)
	problemsMetadata, err := c.ProblemServiceConfig.GetProblemsByFilters(
		ctx,
		problem_service.GetProblemsRequest{
			ProblemIDs: problemIDs,
			PageNumber: 1,
			PageSize:   int32(len(problemIDs)),
		},
	)
	if err != nil {
		return err
	}

	// validate
	for _, id := range problemIDs {
		pmd, ok := problemsMetadata[id]
		if !ok {
			return fmt.Errorf(
				"%w, problem with id %v does not exist",
				flux_errors.ErrInvalidRequest,
				id,
			)
		}

		// if its a public contest, then all the problems in it
		// must have the same lock id
		if contestLockID != nil {
			if pmd.LockID == nil || (*contestLockID != *pmd.LockID) {
				return fmt.Errorf(
					"%w, contest and problems must have the same lock id",
					flux_errors.ErrInvalidRequest,
				)
			}
		}
	}

	return nil
}

func (c *ContestService) unsetContestProblems(
	ctx context.Context,
	qtx *database.Queries,
	contestID uuid.UUID,
) error {
	if qtx == nil {
		return fmt.Errorf(
			"%w, transaction query tool is nil, cannot unset problems of contest with id %v",
			flux_errors.ErrInternal,
			contestID,
		)
	}

	err := qtx.UnsetContestProblems(ctx, contestID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot unset problems of contest with %v, %w",
			flux_errors.ErrInternal,
			contestID,
			err,
		)
		log.Error(err)
		return err
	}

	return nil
}

func (c *ContestService) addProblemsToContest(
	ctx context.Context,
	qtx *database.Queries,
	contestID uuid.UUID,
	problems []ContestProblem,
) (err error) {
	if qtx == nil {
		return fmt.Errorf(
			"%w, transaction query tool is nil, cannot add problems to contest with id %v",
			flux_errors.ErrInternal,
			contestID,
		)
	}

	// check for empty
	if len(problems) == 0 {
		return nil
	}

	for _, problem := range problems {
		_, err = qtx.AddProblemToContest(
			ctx,
			database.AddProblemToContestParams{
				ContestID: contestID,
				ProblemID: problem.ProblemId,
				Score:     problem.Score,
			},
		)
		if err == nil {
			continue
		}

		// if the same question is added multiple times
		// we need to rollback and inform the user as skipping and adding
		// remaining problems will be confusing and creates subtle bugs
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) ||
			pgErr.Code != flux_errors.CodeUniqueConstraintViolation {
			// we don't know the exact case
			err = fmt.Errorf(
				"%w, failed to add problem %d to contest %v: %w",
				flux_errors.ErrInternal,
				problem.ProblemId,
				contestID,
				err,
			)
			log.Error(err)
			return err
		}

		// try sending user readable error
		// constraint name is specified in the db creation sql files
		if pgErr.ConstraintName == "contest_problems_pkey" {
			return fmt.Errorf(
				"%w, duplicate problems found",
				flux_errors.ErrInvalidRequest,
			)
		}

		// fallback
		err = fmt.Errorf(
			"%w, %s",
			flux_errors.ErrInvalidRequest,
			pgErr.Detail,
		)
		return err
	}

	return nil
}

func (c *ContestService) addUsersToContest(
	ctx context.Context,
	qtx *database.Queries,
	contestID uuid.UUID,
	userNames []string,
) (err error) {
	if qtx == nil {
		return fmt.Errorf(
			"%w, transaction query tool is nil, cannot add users to contest with id %v",
			flux_errors.ErrInternal,
			contestID,
		)
	}

	// check for empty
	if len(userNames) == 0 {
		return nil
	}

	// fetch users by filters
	users, err := c.UserServiceConfig.GetUsersByFilters(
		ctx,
		user_service.GetUsersRequest{
			UserNames:  userNames,
			PageNumber: 1,
			PageSize:   int32(len(userNames)),
		},
	)

	usersSet := make(map[string]user_service.UserMetaData)

	// insert users into map
	for _, user := range users {
		usersSet[user.UserName] = user
	}

	// check if all users are present
	for _, userName := range userNames {
		_, ok := usersSet[userName]
		if !ok {
			return fmt.Errorf(
				"%w, user %s does not exist",
				flux_errors.ErrInvalidRequest,
				userName,
			)
		}
	}

	// insert users into db
	for _, user := range usersSet {
		_, err := qtx.RegisterUserToContest(
			ctx, database.RegisterUserToContestParams{
				ContestID: contestID,
				UserID:    user.UserID,
			},
		)
		if err != nil {
			err = fmt.Errorf(
				"%w, cannot register user %s to contest %v, %w",
				flux_errors.ErrInternal,
				user.UserName,
				contestID,
				err,
			)
			log.Error(err)
			return err
		}
	}

	return nil
}

func dbContestToServiceContest(
	dbContest database.GetContestByIDRow,
) (Contest, error) {
	// convert lock access to user_service.UserRole
	var lockAccess *user_service.UserRole
	if dbContest.Access != nil {
		la := user_service.UserRole(*dbContest.Access)
		lockAccess = &la
	}

	// time in database is stored in timezone, convert it to utc
	var utcStartTime *time.Time
	if dbContest.LockID != nil {
		if dbContest.LockTimeout == nil {
			err := fmt.Errorf(
				"%w, lock with id %v has timeout as nil and is associated with contest %v",
				flux_errors.ErrInternal,
				dbContest.LockID,
				dbContest.ID,
			)
			log.Error(err)
			return Contest{}, err
		}

		ust := dbContest.LockTimeout.UTC()
		utcStartTime = &ust
	} else if dbContest.StartTime != nil {
		ust := dbContest.StartTime.UTC()
		utcStartTime = &ust
	} else {
		err := fmt.Errorf(
			"%w, contest %v has both lock_id and start_time as nil",
			flux_errors.ErrInternal,
			dbContest.ID,
		)
		log.Error(err)
		return Contest{}, err
	}

	return Contest{
		ID:          dbContest.ID,
		Title:       dbContest.Title,
		LockId:      dbContest.LockID,
		StartTime:   utcStartTime,
		EndTime:     dbContest.EndTime.UTC(),
		IsPublished: dbContest.IsPublished,
		CreatedBy:   dbContest.CreatedBy,
		LockAccess:  lockAccess,
		LockTimeout: dbContest.LockTimeout,
	}, nil
}

func (c *ContestService) authorizeContestUpdate(
	ctx context.Context,
	contest Contest,
) error {
	// get claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return err
	}

	// any manager can update a public contest
	if contest.LockId != nil {
		err = c.UserServiceConfig.AuthorizeUserRole(
			ctx,
			user_service.RoleManager,
			fmt.Sprintf(
				"user %s tried to update unauthorized public contest with id %v",
				claims.UserName,
				contest.ID,
			),
		)
		return err
	} else {
		// creator access for updating private contest
		err = c.UserServiceConfig.AuthorizeCreatorAccess(
			ctx,
			contest.CreatedBy,
			fmt.Sprintf(
				"user %s tried to update unauthorized private contest with id %v",
				claims.UserName,
				contest.ID,
			),
		)
		if err != nil {
			return err
		}
	}

	// contest cannot be edited once started
	if contest.StartTime != nil {
		if time.Now().After(*contest.StartTime) {
			return fmt.Errorf(
				"%w, cannot perform this action once the contest has started",
				flux_errors.ErrInvalidRequest,
			)
		}
	} else if contest.LockTimeout != nil {
		if time.Now().After(*contest.LockTimeout) {
			return fmt.Errorf(
				"%w, cannot perform this action once the contest has started",
				flux_errors.ErrInvalidRequest,
			)
		}
	} else {
		err = fmt.Errorf(
			"%w, contest has both start time and lockTimeout as nil",
			flux_errors.ErrInternal,
		)
		log.Error(err)
		return err
	}

	return err
}
