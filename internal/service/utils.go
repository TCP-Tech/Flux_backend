package service

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func GenerateSecureRandomInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("min cannot be greater than max")
	}
	diff := big.NewInt(int64(max - min + 1))
	result, err := crand.Int(crand.Reader, diff)
	if err != nil {
		log.Errorf("unable to generate random suffix for usrename, %v", err)
		return 0, errors.Join(flux_errors.ErrInternal, err)
	}
	return int(result.Int64()) + min, nil
}

// custom function for translating validation error into user readable errors
func translateValidationError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", e.Field())
	case "min":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters long", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must be at least %s", e.Field(), e.Param())
	case "max":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters long", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must be at most %s", e.Field(), e.Param())
	case "len":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("%s must be exactly %s characters long", e.Field(), e.Param())
		}
		return fmt.Sprintf("%s must have exactly %s items", e.Field(), e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", e.Field(), e.Param())
	case "numeric":
		return fmt.Sprintf("%s must be a numeric value", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of the following: %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("Validation failed for %s with rule %s", e.Field(), e.Tag())
	}
}

// validateInput validates the input struct using the validator instance on Service.
// If validation fails, it logs and returns the first user-friendly error message.
// Returns nil if input is valid.
func ValidateInput(inp any) error {
	if err := validate.Struct(inp); err != nil {
		var validationErrors validator.ValidationErrors
		// Check if the error is a set of validation errors
		if errors.As(err, &validationErrors) {
			if len(validationErrors) > 0 {
				// Grab and translate the first validation error for user feedback
				errorMessage := translateValidationError(validationErrors[0])
				log.Error(errorMessage)
				// Wrap the error with a custom invalid input error
				return fmt.Errorf("%w, %s", flux_errors.ErrInvalidRequest, errorMessage)
			}
		}
	}
	// All good, input is valid
	return nil
}
