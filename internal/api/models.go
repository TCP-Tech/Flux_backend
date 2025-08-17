package api

import (
	"github.com/tcp_snm/flux/internal/service/auth_service"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
)

type Api struct {
	AuthServiceConfig       *auth_service.AuthService
	ProblemServiceConfig    *problem_service.ProblemService
	LockServiceConfig       *lock_service.LockService
	ContestServiceConfig    *contest_service.ContestService
	TournamentServiceConfig *tournament_service.TournamentService
}
