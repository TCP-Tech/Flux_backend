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

type ContestProblem struct {
	ProblemId int32 `json:"problem_id"`
	Score     int32 `json:"problem_score" validate:"numeric,min=500"`
}

type Contest struct {
	ID                 uuid.UUID        `json:"contest_id"`
	Title              string           `json:"title" validate:"required,min=5,max=100"`
	PickRandomProblems bool             `json:"pick_random_problems"`
	Problems           []ContestProblem `json:"problem_ids"`
	StartTime          time.Time        `json:"start_time"`
	EndTime            time.Time        `json:"end_time"`
	IsPublished        bool             `json:"is_published"`
	
}
