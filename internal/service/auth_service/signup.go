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
	verificationToken string,
) (userResponse UserRegestrationResponse, err error) {
	// verify the token
	if err = a.validateVerificationToken(
		ctx,
		verificationToken,
		userRegestration.UserMail,
		email.PurposeEmailSignUp,
	); err != nil {
		if errors.Is(err, flux_errors.ErrCorruptedVerification) {
			log.WithFields(log.Fields{
				"roll_no": userRegestration.RollNo,
				"purpose": string(email.PurposeEmailSignUp),
				"token":   verificationToken,
			}).Error(err)
		}
		return
	}

	// Validate all inputs together.
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
		verificationToken,
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
		After a large number of tries, api request will generate timeout via ctx.Done()
		Given a small set of users, this startegies suffices for our usecase
	*/
	const maxUserNameRetries = 15
	for i := range maxUserNameRetries {
		attemptLogger := log.WithField("attempt", i+1)
		select {
		case <-ctx.Done():
			err := fmt.Errorf("%w, unable to generate username, %w", flux_errors.ErrInternal, ctx.Err())
			attemptLogger.Error(err)
			return database.User{}, err
		default:
			userName, err := genRandUserName(
				userRegestration.FirstName,
				userRegestration.LastName,
			)
			if err != nil {
				return database.User{}, err
			}
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
				var pgErr *pgconn.PgError
				if errors.As(dbErr, &pgErr) && pgErr.Code == flux_errors.CodeUniqueConstraintViolation {
					if strings.Contains(pgErr.ConstraintName, "user_name") {
						attemptLogger.Errorf("cannot create user. user_name %s already exist", userName)
						continue
					}
					return database.User{}, fmt.Errorf("%s %w", pgErr.Detail, flux_errors.ErrUserAlreadyExists)
				}
				attemptLogger.Errorf("failed to insert user into database: %v", dbErr)
				return database.User{}, errors.Join(flux_errors.ErrInternal, dbErr)
			}
			return user, nil
		}
	}
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
