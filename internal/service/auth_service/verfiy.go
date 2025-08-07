package auth_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultTokenExpiryMinutes = 15
)

func (a *AuthService) SendVerificationEmail(
	ctx context.Context,
	userEmail string,
	verifyPurpose email.EmailPurpose,
) error {
	// validate the email
	if err := service.ValidateInput(
		// pass anonymous structs to verify indpendent fields
		struct {
			Email string `json:"email" validate:"required,email"`
		}{
			userEmail,
		},
	); err != nil {
		return err
	}

	// create a new token
	plainToken, err := generateToken()
	if err != nil {
		return err
	}

	// hash the plainToken
	hashToken, err := bcrypt.GenerateFromPassword([]byte(plainToken), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("failed to create bcrypt token, %v", err)
		return errors.Join(flux_errors.ErrInternal, err)
	}

	// create a new token in db
	err = a.createTokenInDb(ctx, userEmail, verifyPurpose, string(hashToken))
	if err != nil {
		return err
	}

	// send mail
	switch verifyPurpose {
	case email.PurposeEmailSignUp:
		err = email.NewMail(
			ctx,
			"Verify your flux user account",
			fmt.Sprintf("token to verify your flux accout, %v", plainToken),
			email.KeyEmailBodyPlain,
			verifyPurpose,
			userEmail,
		)
	case email.PurposeEmailPasswordReset:
		err = email.NewMail(
			ctx,
			"Flux account password reset",
			fmt.Sprintf(
				"token to reset your password: %s, if this is not you please inform.",
				plainToken,
			),
			email.KeyEmailBodyPlain,
			verifyPurpose,
			userEmail,
		)
	}

	return err
}

func (a *AuthService) createTokenInDb(
	ctx context.Context,
	userEmail string,
	purpose email.EmailPurpose,
	hashedToken string,
) error {
	// create expiry field
	expiry := time.Now().Add(DefaultTokenExpiryMinutes * time.Minute)

	// create a new token
	_, err := a.DB.CreateToken(
		ctx, database.CreateTokenParams{
			HashedToken: hashedToken,
			Purpose:     string(purpose),
			ExpiresAt:   expiry,
			Email:       userEmail,
			Payload:     json.RawMessage("{}"),
		},
	)

	if err != nil {
		log.Errorf("unable to create a verification token in db, %v", err)
		return errors.Join(flux_errors.ErrInternal, err)
	}

	return nil
}

// validate the verification token sent by user to verify email
func (a *AuthService) validateVerificationToken(
	ctx context.Context,
	token string,
	userMail string,
	purpose email.EmailPurpose,
) error {
	dbToken, err := a.verifyToken(ctx, token, userMail, purpose)
	if err != nil {
		return err
	}

	// --- any extra verification ---

	// check the expiry and if expired invalidate the token
	if time.Now().After(dbToken.ExpiresAt) {
		// create a logging helper
		invLogger := log.WithFields(
			log.Fields{
				"purpose": string(email.PurposeEmailSignUp),
				"token":   token,
			},
		)

		// invalidate the token
		if invErr := a.invalidateVerificationToken(
			ctx,
			token,
			userMail,
			email.PurposeEmailSignUp,
		); invErr != nil {
			// failed to invalidate. log the error
			invLogger.Errorf("failed to invalidate expired token, %v", invErr)
		} else {
			invLogger.Info("invalidated expired token")
		}

		// return expiration error
		return fmt.Errorf(
			"%w, %w",
			flux_errors.ErrInvalidRequest,
			flux_errors.ErrVerificationTokenExpired,
		)
	}

	return nil
}

func (a *AuthService) invalidateVerificationToken(
	ctx context.Context,
	token string,
	userMail string,
	purpose email.EmailPurpose,
) error {
	_, err := a.verifyToken(ctx, token, userMail, purpose)
	if err != nil {
		return err
	}

	err = a.DB.DeleteByEmailAndPurpose(ctx, database.DeleteByEmailAndPurposeParams{
		Email:   userMail,
		Purpose: string(purpose),
	})

	if err != nil {
		log.Errorf("unable to invalidate token, %v", err)
		err = errors.Join(flux_errors.ErrInternal, err)
	}

	log.WithFields(log.Fields{
		"token":   token,
		"purpose": string(purpose),
	}).Info("invalidated token")
	return err
}

/*
Check if the latest token inserted in db for a specific email and purpose matches the token
*/
func (a *AuthService) verifyToken(
	ctx context.Context,
	token string,
	userMail string,
	purpose email.EmailPurpose,
) (dbToken database.Token, err error) {
	// validate email
	if userMail == "" {
		err = fmt.Errorf("%w, invalid email provided", flux_errors.ErrCorruptedVerification)
		return
	}

	// get token from db
	dbToken, err = a.DB.GetTokenByEmailAndPurpose(
		ctx,
		database.GetTokenByEmailAndPurposeParams{
			Email:   userMail,
			Purpose: string(purpose),
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf(
				"%w, invalid token or email provided",
				flux_errors.ErrCorruptedVerification,
			)
			return
		}
		log.Errorf("unable to retrieve token data from db, %v", err)
		err = errors.Join(flux_errors.ErrInternal, err)
		return
	}

	// check if token is correct
	err = bcrypt.CompareHashAndPassword([]byte(dbToken.HashedToken), []byte(token))
	if err != nil {
		log.Infof("invalid token. failed to match token hash and token, %v", err)
		err = fmt.Errorf("%w, please cross check your token", flux_errors.ErrCorruptedVerification)
		return
	}

	return
}
