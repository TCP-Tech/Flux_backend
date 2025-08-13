package contest_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

func (c *ContestService) GetContestRegisteredUsers(
	ctx context.Context,
	contestID uuid.UUID,
) ([]user_service.UserMetaData, error) {
	// fetch userIDs from db
	userIDs, err := c.DB.GetContestUsers(ctx, contestID)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch users of contest %v, %w",
			flux_errors.ErrInternal,
			contestID,
			err,
		)
		log.Error(err)
		return nil, err
	}

	// check for empty
	if len(userIDs) == 0 {
		return make([]user_service.UserMetaData, 0), nil
	}

	// fetch users from db by using userIDs
	users, err := c.UserServiceConfig.GetUsersByFilters(
		ctx,
		user_service.GetUsersRequest{
			PageNumber: 1,
			PageSize: int32(len(userIDs)),
			UserIDs: userIDs,
		},
	)

	// if fetched users and userIDs are not same, log the difference
	if len(users) != len(userIDs) {
		log.WithFields(
			log.Fields{
				"registered_user_ids": userIDs,
				"fetched_users": users,
			},
		).Warnf(
			"contest registered userIDs and fetched_users of contest %v are not same",
			contestID,
		)
	}

	return users, err
}