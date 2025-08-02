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
	RandomProblems          int              `json:"random_problems" validate:"numeric,min=0"`
	RandomProblemsMinRating int              `json:"random_problems_min_rating" validate:"numeric,min=800"`
	RandomProblemsMaxRating int              `json:"random_problems_max_rating" validate:"numeric,min=3000"`
	Problems                []ContestProblem `json:"problems"`
}

type ContestProblem struct {
	ProblemId int32 `json:"problem_id"`
	Score     int32 `json:"problem_score" validate:"numeric,min=500"`
}

type Contest struct {
	ID          uuid.UUID `json:"contest_id"`
	Title       string    `json:"title" validate:"required,min=5,max=100"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsPublished bool      `json:"is_published"`
}
