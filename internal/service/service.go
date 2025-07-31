package service

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
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
	validate *validator.Validate
)

func InitializeServices() {
	validate = initValidator() // used for validating struct fields
}

func initValidator() *validator.Validate {
	log.Info("initializing validator")
	validate := validator.New(validator.WithRequiredStructEnabled())

	// This makes error.Field() return "first_name" instead of "FirstName"
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return validate
}
