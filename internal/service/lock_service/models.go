package lock_service

import (
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type LockService struct {
	DB                *database.Queries
	UserServiceConfig *user_service.UserService
}

type FluxLock struct {
	ID          uuid.UUID             `json:"lock_id"`
	Name        string                `json:"name" validate:"min=4"`
	CreatedBy   uuid.UUID             `json:"created_by"`
	Type        database.LockType     `json:"lock_type" validate:"oneof=timer manual"`
	CreatedAt   time.Time             `json:"created_at"`
	Timeout     *time.Time            `json:"timeout"`
	Description string                `json:"description"`
	Access      user_service.UserRole `json:"-"`
}

type GetLocksRequest struct {
	LockName        string  `json:"lock_name"`
	CreatorUserName string  `json:"creator_user_name"`
	CreatorRollNo   string  `json:"creator_roll_no"`
	PageNumber      int32   `json:"page_number" validate:"min=1,numeric"`
	PageSize        int32   `json:"page_size" validate:"min=1,max=100,numeric"`
}
