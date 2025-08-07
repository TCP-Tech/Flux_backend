package flux_errors

import (
	"errors"
)

const (
	CodeUniqueConstraintViolation = "23505"
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
)
