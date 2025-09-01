package auth_service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (a *AuthService) SignUp(
	ctx context.Context,
	userRegestration UserRegestration,
) (userResponse UserRegestrationResponse, err error) {
	// verify the token
	if err = a.validateVerificationToken(
		ctx,
		userRegestration.VerificationToken,
		userRegestration.UserMail,
		email.PurposeEmailSignUp,
	); err != nil {
		if errors.Is(err, flux_errors.ErrCorruptedVerification) {
			log.WithFields(log.Fields{
				"roll_no": userRegestration.RollNo,
				"purpose": string(email.PurposeEmailSignUp),
				"token":   userRegestration.VerificationToken,
			}).Warn(err)
		}
		return
	}

	// Validate
	if err = service.ValidateInput(userRegestration); err != nil {
		return
	}

	// Hash the password.
	passwordHash, err := generatePasswordHash(userRegestration.Password)
	if err != nil {
		return
	}

	// Create the user in the database and handle DB-specific errors.
	dbUser, err := a.createUserInDB(ctx, userRegestration, passwordHash)
	if err != nil {
		return
	}

	// invalidate verification token
	if err = a.invalidateVerificationToken(
		ctx,
		userRegestration.VerificationToken,
		userRegestration.UserMail,
		email.PurposeEmailSignUp,
	); err != nil {
		return
	}

	// Log and return
	log.WithFields(log.Fields{
		"user_name": dbUser.UserName,
		"roll_no":   dbUser.RollNo,
	}).Info("created user")

	userResponse = userRegToResponse(dbUser.UserName, userRegestration)

	return
}

// --- Helper Functions Below ---
// createUserInDB handles the database interaction and error-specific logic.
func (a *AuthService) createUserInDB(
	ctx context.Context,
	userRegestration UserRegestration,
	passwordHash string,
) (database.User, error) {
	/*
		Generate a random userName and try to insert into db.
		If username already exist, try to create a new one.
		Either after max retries or ctx.Done() return failure
		Given a small set of users, this startegy suffices for our usecase
	*/
	const maxUserNameRetries = 15
	for i := range maxUserNameRetries {
		attemptLogger := log.WithField("attempt", i+1)
		select {
		// if request timelimit is exceeded, return failure
		case <-ctx.Done():
			err := fmt.Errorf("%w, unable to generate username, %w", flux_errors.ErrInternal, ctx.Err())
			attemptLogger.Error(err)
			return database.User{}, err
		default:
			// generate a random user_name
			userName, err := genRandUserName(
				userRegestration.FirstName,
				userRegestration.LastName,
			)
			if err != nil {
				return database.User{}, err
			}

			// create user in db
			user, dbErr := a.DB.CreateUser(
				ctx,
				database.CreateUserParams{
					UserName:     userName,
					PasswordHash: passwordHash,
					RollNo:       userRegestration.RollNo,
					FirstName:    userRegestration.FirstName,
					LastName:     userRegestration.LastName,
					Email:        userRegestration.UserMail,
				},
			)
			if dbErr != nil {
				// if user_name already exist retry
				var pgErr *pgconn.PgError
				if errors.As(dbErr, &pgErr) &&
					pgErr.ConstraintName == "uq_users_user_name" {
					attemptLogger.Errorf("cannot create user. user_name %s already exist", userName)
					continue
				}
				// other db error
				dbErr = flux_errors.HandleDBErrors(
					dbErr,
					errMsgs,
					"failed to insert user into db",
				)
				return user, dbErr
			}

			// user created successfully, return created user
			return user, nil
		}
	}

	// failed to create user after max retries
	err := fmt.Errorf("%w, unable to create user. max retries exceeded", flux_errors.ErrInternal)
	log.Error(err)
	return database.User{}, err
}

func genRandUserName(firstName string, lastName string) (string, error) {
	// strategy may change
	minRandNum := 234
	maxRandNum := 789
	suffix, err := service.GenerateSecureRandomInt(minRandNum, maxRandNum)
	if err != nil {
		return "", err
	}
	name := strings.ToLower("flux#" + lastName + firstName[:3] + strconv.Itoa(suffix))
	return name, nil
}
