package user_service

import (
	"context"
	"fmt"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (u *UserService) GetUserByUserNameOrRollNo(
	ctx context.Context,
	userName string,
	rollNo string,
) (dbUser database.User, err error) {
	if userName == "" && rollNo == "" {
		err = fmt.Errorf("%w, either user_name or roll_no must be provided", flux_errors.ErrInvalidRequest)
		return
	}
	if userName != "" {
		dbUser, err = u.FetchUserByUserName(ctx, userName)
	} else {
		dbUser, err = u.FetchUserByRollNo(ctx, rollNo)
	}
	return
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
