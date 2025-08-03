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
	ID          uuid.UUID `json:"lock_id"`
	Name        string    `json:"name" validate:"min=4"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	Timeout     time.Time `json:"timeout"`
	Description string    `json:"description"`
}
