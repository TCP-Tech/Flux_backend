package user_service

import "github.com/tcp_snm/flux/internal/database"

type UserService struct {
	DB *database.Queries
}

type UserRole string

const (
	RoleManager UserRole = "role_manager"
	RoleHC      UserRole = "role_hc"
)

// this type must be used only when a 
// "multiple" users are being passed
type RawUser struct {
	UserName string `json:"user_name"`
	RollNo   string `json:"roll_no"`
}
