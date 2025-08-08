package cotnest_service

import (
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type ContestService struct {
	DB                   *database.Queries
	UserServiceConfig    *user_service.UserService
	ProblemServiceConfig *problem_service.ProblemService
}

type ContestProblems struct {
	Problems []ContestProblem `json:"problems"`
}

type ContestProblem struct {
	ProblemId int32 `json:"problem_id"`
	Score     int32 `json:"problem_score" validate:"numeric,min=500"`
}

type Contest struct {
	ID          uuid.UUID `json:"contest_id"`
	Title       string    `json:"title" validate:"required,min=5,max=100"`
	LockId      uuid.UUID `json:"lock_id"`
	EndTime     time.Time `json:"end_time"`
	IsPublished bool      `json:"is_published"`
}
