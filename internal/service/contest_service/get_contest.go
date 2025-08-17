package contest_service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (c *ContestService) GetContestByID(
	ctx context.Context,
	id uuid.UUID,
) (Contest, error) {
	// get contest
	dbContest, err := c.DB.GetContestByID(
		ctx,
		id,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Contest{}, fmt.Errorf(
				"%w, contest with id %v does not exist",
				flux_errors.ErrInvalidRequest,
				id,
			)
		}
		err = fmt.Errorf(
			"%w, cannot fetch contest with id %v from db, %w",
			flux_errors.ErrInternal,
			id,
			err,
		)
		log.Error(err)
		return Contest{}, err
	}

	return dbContestToServiceContest(dbContest)
}

func (c *ContestService) GetContestsByFilters(
	ctx context.Context,
	request GetContestRequest,
) ([]Contest, error) {
	// validate
	err := service.ValidateInput(request)
	if err != nil {
		return nil, err
	}

	offset := (request.PageNumber - 1) * request.PageSize

	// get contests
	dbContests, err := c.DB.GetContestsByFilters(
		ctx,
		database.GetContestsByFiltersParams{
			ContestIds:  request.ContestIDs,
			IsPublished: request.IsPublished,
			LockID:      request.LockID,
			TitleSearch: request.TitleSearch,
			Limit:       request.PageSize,
			Offset:      offset,
		},
	)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch contest with filters %v, %w",
			flux_errors.ErrInternal,
			request,
			err,
		)
		log.Error(err)
		return nil, err
	}

	res := make([]Contest, 0, len(dbContests))

	// check for empty
	if len(dbContests) == 0 {
		return res, nil
	}

	// convert
	for _, dbContest := range dbContests {
		// calculate start time
		var startTime *time.Time
		if dbContest.StartTime != nil {
			startTime = dbContest.StartTime
		} else if dbContest.LockTimeout != nil {
			startTime = dbContest.LockTimeout
		} else {
			log.Warnf(
				"contest with id %v has both start time and lock time as nil",
				dbContest.ID,
			)
			continue
		}
		utcStartTime := startTime.UTC()

		var lockAccess *user_service.UserRole
		if dbContest.LockAccess != nil {
			la := user_service.UserRole(*dbContest.LockAccess)
			lockAccess = &la
		}

		contest := Contest{
			ID:          dbContest.ID,
			Title:       dbContest.Title,
			LockId:      dbContest.LockID,
			StartTime:   &utcStartTime,
			EndTime:     dbContest.EndTime.UTC(),
			IsPublished: dbContest.IsPublished,
			CreatedBy:   dbContest.CreatedBy,
			LockAccess:  lockAccess,
			LockTimeout: dbContest.LockTimeout,
		}

		res = append(res, contest)
	}

	return res, nil
}

func (c *ContestService) GetUserRegisteredContests(
	ctx context.Context,
	pageNumber int32,
	pageSize int32,
) ([]Contest, error) {
	// get user id from claims
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// get the contest ids
	contestIDs, err := c.DB.GetUserRegisteredContests(ctx, claims.UserId)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch registered contests of user %s",
			flux_errors.ErrInternal,
			claims.UserName,
		)
		log.Error(err)
		return nil, err
	}

	// check for empty
	if len(contestIDs) == 0 {
		return make([]Contest, 0), nil
	}

	// fetch contests using filters
	contests, err := c.GetContestsByFilters(
		ctx,
		GetContestRequest{
			ContestIDs: contestIDs,
			PageNumber: pageNumber,
			PageSize:   pageSize,
		},
	)

	return contests, err
}
