package flux_errors

import (
	"database/sql"
	"errors"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
	log "github.com/sirupsen/logrus"
)

const (
	CodeUniqueConstraint     = "23505"
	CodeForeignKeyConstraint = "23503"
)

var (
	ErrInternal                  = errors.New("internal service error. please try again later")
	ErrInvalidRequest            = errors.New("invalid request")
	ErrUserAlreadyExists         = errors.New("some other user has already taken that key")
	ErrInvalidUserCredentials    = errors.New("invalid username or roll_no and password")
	ErrInvalidRequestCredentials = errors.New("invalid request credentials")
	ErrEmailServiceStopped       = errors.New("email service is stopped currently")
	ErrVerificationTokenExpired  = errors.New("verfication token expired. please try again")
	ErrCorruptedVerification     = errors.New("corrupted verificaiton")
	ErrUnAuthorized              = errors.New("user not allowed to perform this action")
	ErrNotFound                  = errors.New("entity not found")
	ErrPartialResult             = errors.New("unable to fetch complete list of requested entities")
	ErrTaskLaunchError           = errors.New("failed to launch task")
	ErrTaskSIGTERM               = errors.New("task failed to terminate gracefully")
	ErrTaskKill                  = errors.New("cannot kill task")
	ErrSubmissionFailed          = errors.New("submission failed")
	ErrWaitAlreadyCalled         = errors.New("exec: Wait was already called")
	ErrHttpResponse              = errors.New("error occurred with http response")
	ErrMonitorStart              = errors.New("monitor failed to start")
)

func HandleDBErrors(
	err error,
	errMsgs map[string]map[string]string,
	fallBackMsg string,
) error {
	// assume its an internal error first
	err = fmt.Errorf(
		"%w, %s, %w",
		ErrInternal,
		fallBackMsg,
		err,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf(
			"%w, entity with given id doesn't exist",
			ErrNotFound,
		)
	}

	// check if its a pg error
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		log.Error(err)
		return err
	}

	if errMsgs == nil {
		log.Warnf("go null errMsgs")
		log.Error(err)
		return err
	}

	// check if its a foriegn key error
	if pgErr.Code == CodeForeignKeyConstraint {
		msgForeignKey, ok := errMsgs[CodeForeignKeyConstraint]
		if !ok {
			log.Warnf("no msg map found for foreign key constraint.")
			return fmt.Errorf(
				"%w, %s",
				ErrInvalidRequest,
				pgErr.Detail,
			)
		}
		return HandleForeignKeyError(pgErr, msgForeignKey)
	}

	// check if its a unique key error
	if pgErr.Code == CodeUniqueConstraint {
		msgUniqueConstraint, ok := errMsgs[CodeUniqueConstraint]
		if !ok {
			log.Warnf("no msg map found for unique key constraint.")
			return fmt.Errorf(
				"%w, %s",
				ErrInvalidRequest,
				pgErr.Detail,
			)
		}
		return HandleUniqueKeyError(pgErr, msgUniqueConstraint)
	}

	// unknown error
	log.Error(err)
	return err
}

func HandleForeignKeyError(pgErr *pgconn.PgError, msgForeignKey map[string]string) error {
	msg, ok := msgForeignKey[pgErr.ConstraintName]
	if !ok {
		log.Warnf(
			"unknown foreign key violation, %s occured while inserting standard_problem_data",
			pgErr.ConstraintName,
		)
		msg = pgErr.Detail
	}
	err := fmt.Errorf(
		"%w, %s",
		ErrInvalidRequest,
		msg,
	)
	return err
}

func HandleUniqueKeyError(pgErr *pgconn.PgError, msgUniqueConstraint map[string]string) error {
	msg, ok := msgUniqueConstraint[pgErr.ConstraintName]
	if !ok {
		log.Warnf(
			"unknown unique key violation, %s occured while inserting standard_problem_data",
			pgErr.ConstraintName,
		)
		msg = pgErr.Detail
	}
	err := fmt.Errorf(
		"%w, %s",
		ErrInvalidRequest,
		msg,
	)
	return err
}

// handles inter process communication errors
func HandleIPCError(err error) error {
	var opError *net.OpError
	if errors.As(err, &opError) {
		err = fmt.Errorf(
			"%w, \"%s\" error occurred during \"%s\" operation with %s",
			ErrInternal,
			opError.Error(),
			opError.Op,
			opError.Addr,
		)
		log.WithFields(
			log.Fields{
				"network":        opError.Net,
				"source address": opError.Source,
			},
		).Error(err)
		return err
	}

	// unknown error
	err = fmt.Errorf(
		"%w, %w", ErrInternal, err,
	)
	log.Error(err)
	return err
}
