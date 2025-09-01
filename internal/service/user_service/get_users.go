package user_service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (u *UserService) GetUserProfile(
	ctx context.Context,
	userID uuid.UUID,
) (User, error) {
	// get user from db
	dbUser, err := u.DB.GetUserById(ctx, userID)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			fmt.Sprintf("cannot fetch user with id %v from db", userID),
		)
		return User{}, err
	}

	// convert and return
	return User{
		ID:           dbUser.ID,
		RollNo:       dbUser.RollNo,
		UserName:     dbUser.UserName,
		FirstName:    dbUser.FirstName,
		LastName:     dbUser.LastName,
		Email:        dbUser.Email,
		PasswordHash: dbUser.PasswordHash,
	}, nil
}

func (u *UserService) GetMe(ctx context.Context) (User, error) {
	claims, err := service.GetClaimsFromContext(ctx)
	if err != nil {
		return User{}, err
	}

	return u.GetUserProfile(ctx, claims.UserId)
}

func (u *UserService) GetUsersByFilters(
	ctx context.Context,
	request GetUsersRequest,
) ([]UserMetaData, error) {
	// validate
	err := service.ValidateInput(request)
	if err != nil {
		return nil, err
	}

	// calc offset
	offset := (request.PageNumber - 1) * request.PageSize

	// fetch users
	dbUsers, err := u.DB.GetUsersByFilters(ctx, database.GetUsersByFiltersParams{
		UserIds:   request.UserIDs,
		UserNames: request.UserNames,
		RollNos:   request.RollNos,
		Limit:     request.PageSize,
		Offset:    offset,
	})
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot fetch users by filters from db",
			flux_errors.ErrInternal,
		)
		log.WithField("request", request).Error(err)
		return nil, err
	}

	// convert to user metadata
	res := make([]UserMetaData, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		user := UserMetaData{
			UserName: dbUser.UserName,
			RollNo:   dbUser.RollNo,
			UserID:   dbUser.ID,
		}
		res = append(res, user)
	}

	return res, nil
}
