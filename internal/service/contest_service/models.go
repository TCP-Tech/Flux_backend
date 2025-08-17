package contest_service

import (
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type ContestService struct {
	DB                   *database.Queries
	UserServiceConfig    *user_service.UserService
	LockServiceConfig    *lock_service.LockService
	ProblemServiceConfig *problem_service.ProblemService
}

type ContestProblem struct {
	ProblemId int32 `json:"problem_id"`
	Score     int32 `json:"score" validate:"min=0"`
}

type ContestProblemResponse struct {
	ProblemData problem_service.ProblemMetaData `json:"problem_id"`
	Score       int32                           `json:"score" validate:"min=0"`
}

// used in requests
type Contest struct {
	ID          uuid.UUID  `json:"contest_id"`
	Title       string     `json:"title" validate:"required,min=5,max=100"`
	LockId      *uuid.UUID `json:"lock_id"`
	StartTime   *time.Time `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	IsPublished bool       `json:"is_published"`
	CreatedBy   uuid.UUID  `json:"created_by"`

	// fields used only for internal purpose
	LockAccess  *user_service.UserRole `json:"-"`
	LockTimeout *time.Time             `json:"-"`
}

type CreateContestRequest struct {
	ContestDetails  Contest          `json:"contest_details"`
	RegisteredUsers []string         `json:"user_names"`
	ContestProblems []ContestProblem `json:"problems"`
}

type GetContestRequest struct {
	ContestIDs  []uuid.UUID `json:"contest_ids"`
	IsPublished *bool       `json:"is_published"`
	LockID      *uuid.UUID  `json:"lock_id"`
	TitleSearch string      `json:"title"`
	PageNumber  int32       `json:"page_number" validate:"min=1,max=10000"`
	PageSize    int32       `json:"page_size" validate:"min=0,max=10000"`
}
