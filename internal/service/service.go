package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/oleiade/lane"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

type contextKey string

const (
	MinUsernameLength               = 5
	MinPasswordLength               = 10
	MaxPasswordLength               = 74
	KeyJWTSecret                    = "JWT_SECRET"
	KeyUserName                     = "user_name"
	KeyRollNo                       = "roll_no"
	KeyExp                          = "exp"
	KeyIAt                          = "iat"
	KeyCtxUserCredClaims contextKey = "UserCredClaims"
)

var (
	validate  *validator.Validate
	pool      *pgxpool.Pool
	waitQueue *lane.PQueue
)

func InitializeServices(mainPool *pgxpool.Pool) {
	validate = initValidator() // used for validating struct fields
	pool = mainPool
	waitQueue = lane.NewPQueue(lane.MINPQ)
	go processWaitQueue()
}

func initValidator() *validator.Validate {
	log.Info("initializing go/playground service validator")
	validate := validator.New(validator.WithRequiredStructEnabled())

	// This makes error.Field() return sanke case. for eg., "first_name" instead of "FirstName"
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return validate
}

func GetClaimsFromContext(
	ctx context.Context,
) (claims UserCredentialClaims, err error) {
	claimsValue := ctx.Value(KeyCtxUserCredClaims)
	claims, ok := claimsValue.(UserCredentialClaims)
	if !ok {
		err = fmt.Errorf(
			"%w, unable to parse claims to auth_service.UserCredentialClaims, type of claims found is %T",
			flux_errors.ErrInternal,
			reflect.TypeOf(claims),
		)
		log.Error(err)
	}
	return
}

// getNewTransaction starts a new database transaction using the connection pool.
func GetNewTransaction(
	ctx context.Context,
) (pgx.Tx, error) {
	// Begin a new transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		// Wrap and log the error if transaction creation fails
		err = fmt.Errorf(
			"%w, cannot create transaction, %w",
			flux_errors.ErrInternal,
			err,
		)
		log.Error(err)
		return nil, err
	}

	// Return the transaction object and nil error
	return tx, err
}

func AddToWaitQueue(waitElement WaitElement) {
	// calculate wakeup time
	priority := time.Now().Add(
		time.Millisecond * time.Duration(waitElement.DelayMS),
	).UnixMilli()
	
	waitQueue.Push(waitElement, int(priority))
}

func processWaitQueue() {
	for {
		// sleep to avoid infinite continuous iterations
		time.Sleep(time.Millisecond * 50)

		for waitQueue.Size() > 0 {
			_, expiry := waitQueue.Head()
			if !isExpired(int64(expiry)) {
				break
			}
			// pop and process
			rawElement, _ := waitQueue.Pop()
			waitElement, ok := rawElement.(WaitElement)
			if !ok {
				log.Errorf("cannot csat %v to WaitElement", rawElement)
				continue
			}
			// launch it as a goroutine for non-blocking
			go waitElement.Process(waitElement.Element)
		}
	}
}

func isExpired(unixMilli int64) bool {
	t := time.UnixMilli(unixMilli)
	return time.Now().After(t)
}
