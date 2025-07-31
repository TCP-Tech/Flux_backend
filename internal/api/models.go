package api

import (
	auth_service "github.com/tcp_snm/flux/internal/service/auth"
	problem_service "github.com/tcp_snm/flux/internal/service/problems"
)

type Api struct {
	AuthServiceConfig    *auth_service.AuthService
	ProblemServiceConfig *problem_service.ProblemService
}
