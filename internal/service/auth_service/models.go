package auth_service

import (
	"fmt"

	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

var (
	msgUniqueKey = map[string]string{
		"uq_users_roll_no": "user with that roll_no already exist",
		"uq_users_email":   "user with that email already exist",
	}

	errMsgs = map[string]map[string]string{
		flux_errors.CodeUniqueConstraint: msgUniqueKey,
	}
)

type AuthService struct {
	DB         *database.Queries
	UserConfig *user_service.UserService
}

type UserRegestration struct {
	FirstName         string `json:"first_name" validate:"required,min=4"`
	LastName          string `json:"last_name" validate:"required,min=4"`
	RollNo            string `json:"roll_no" validate:"required,len=8,numeric"`
	Password          string `json:"password" validate:"required,min=7,max=20"`
	UserMail          string `json:"email" validate:"required,email"`
	VerificationToken string `json:"verification_token"`
}

type UserRegestrationResponse struct {
	UserName string `json:"user_name"`
	RollNo   string `json:"roll_no"`
}

type UserLoginRequest struct {
	UserName         *string `json:"user_name"`
	RollNo           *string `json:"roll_no"`
	Password         string  `json:"password"`
	RememberForMonth bool    `json:"remember_for_month"`
}

type ResetPasswordRequest struct {
	UserName *string `json:"user_name"`
	RollNo   *string `json:"roll_no"`
	Password string  `json:"password"`
	Token    string  `json:"verification_token"`
}

func (rpr ResetPasswordRequest) String() string {
	uns := "<nil>"
	if rpr.UserName != nil {
		uns = *rpr.UserName
	}
	rns := "<nil>"
	if rpr.RollNo != nil {
		rns = *rpr.RollNo
	}
	return fmt.Sprintf("user_name=%s, roll_no=%s", uns, rns)
}
