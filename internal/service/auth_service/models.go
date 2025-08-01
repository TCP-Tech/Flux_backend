package auth_service

import (
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type AuthService struct {
	DB         *database.Queries
	UserConfig *user_service.UserService
}

type UserRegestration struct {
	FirstName string `json:"first_name" validate:"required,min=4"`
	LastName  string `json:"last_name" validate:"required,min=4"`
	RollNo    string `json:"roll_no" validate:"required,len=8,numeric"`
	// password greater than 74 characters in length cannot be hashed
	Password string `json:"password" validate:"required,min=7,max=74"`
	UserMail string `json:"email" validate:"required,email"`
}

type UserRegestrationResponse struct {
	UserName string `json:"user_name"`
	RollNo   string `json:"roll_no"`
}

type UserLoginResponse struct {
	UserName  string
	RollNo    string
	FirstName string
	LastName  string
	Roles     []string
}
