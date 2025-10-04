package auth_service

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (a *AuthService) ResetPasswordSendMail(
	ctx context.Context,
	userName *string,
	rollNo *string,
) error {
	// fetch the user from db
	userMetaData, err := a.UserConfig.GetUserByUserNameOrRollNo(ctx, userName, rollNo)
	if err != nil {
		return err
	}
	user, err := a.UserConfig.GetUserProfile(ctx, userMetaData.UserID)
	if err != nil {
		return err
	}

	// send verification mail
	err = a.SendVerificationEmail(
		ctx,
		user.Email,
		email.PurposeEmailPasswordReset,
	)

	return err
}

func (a *AuthService) ResetPassword(
	ctx context.Context,
	request ResetPasswordRequest,
) error {
	// create a custom logger
	resetLogger := log.WithFields(
		log.Fields{
			"request": request,
			"purpose": string(email.PurposeEmailPasswordReset),
		},
	)

	// fetch user from db
	userMetaData, err := a.UserConfig.GetUserByUserNameOrRollNo(
		ctx, request.UserName, request.RollNo,
	)
	if err != nil {
		return err
	}
	user, err := a.UserConfig.GetUserProfile(ctx, userMetaData.UserID)
	if err != nil {
		return err
	}

	// verify token
	if err = a.validateVerificationToken(
		ctx,
		request.Token,
		user.Email,
		email.PurposeEmailPasswordReset,
	); err != nil {
		if errors.Is(err, flux_errors.ErrCorruptedVerification) {
			resetLogger.Error(err)
		}
		return err
	}

	// validate password
	if err = service.ValidateInput(
		struct {
			Password string `json:"password" validate:"required,min=7,max=74"`
		}{
			Password: request.Password,
		},
	); err != nil {
		return err
	}

	// generate password hash
	passwordHash, err := generatePasswordHash(request.Password)
	if err != nil {
		return err
	}

	// insert into db
	if err = a.DB.ResetPassword(ctx, database.ResetPasswordParams{
		PasswordHash: passwordHash,
		UserName:     user.UserName,
	}); err != nil {
		resetLogger.Errorf("unable to reset password, %v", err)
		err = fmt.Errorf("%w, unable to reset password", flux_errors.ErrInternal)
		return err
	}

	// invalidate token
	a.invalidateVerificationToken(
		ctx,
		request.Token,
		user.Email,
		email.PurposeEmailPasswordReset,
	)

	return nil
}
