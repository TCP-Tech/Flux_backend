package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/tcp_snm/flux/middleware"
)

func NewV1Router() *chi.Mux {
	v1 := chi.NewRouter()

	// configure all endpoints
	v1.Get("/healthz", middleware.JWTMiddleware(apiConfig.HandlerReadiness))

	// auth layer
	v1.Get("/auth/signup", apiConfig.HandlerSignUpSendMail)
	v1.Post("/auth/signup", apiConfig.HandlerSignUp)
	v1.Post("/auth/login", apiConfig.HandlerLogin)
	v1.Get("/auth/reset-password", apiConfig.HandlerResetPasswordSendMail)
	v1.Post("/auth/reset-password", apiConfig.HandlerResetPassword)

	// locks layer
	// get locks
	v1.Get("/locks", middleware.JWTMiddleware(apiConfig.HandlerGetLockById))
	v1.Post("/locks/search", middleware.JWTMiddleware(apiConfig.HandlerGetLocksByFilter))
	// create lock
	v1.Post("/locks", middleware.JWTMiddleware(apiConfig.HandlerCreateLock))
	// update lock
	v1.Put("/locks", middleware.JWTMiddleware(apiConfig.HandlerUpdateLock))
	// delete lock
	v1.Delete("/locks", middleware.JWTMiddleware(apiConfig.HanlderDeleteLockById))

	// problems layer
	// search
	v1.Get("/problems", middleware.JWTMiddleware(apiConfig.HandlerGetProblemById))
	v1.Post("/problems/search", middleware.JWTMiddleware(apiConfig.HandlerGetProblemsByFilters))
	// add
	v1.Post("/problems", middleware.JWTMiddleware(apiConfig.HandlerAddProblem))
	// update
	v1.Put("/problems", middleware.JWTMiddleware(apiConfig.HandlerUpdateProblem))

	// contest
	// search
	v1.Get("/contests", middleware.JWTMiddleware(apiConfig.HandlerGetContestByID))
	v1.Get("/contests/problems", middleware.JWTMiddleware(apiConfig.HandlerGetContestProblems))
	v1.Get("/contests/users", middleware.JWTMiddleware(apiConfig.HandlerGetContestUsers))
	v1.Post("/contests/search", middleware.JWTMiddleware(apiConfig.HandlerGetContestsByFilters))
	v1.Get("/contests/user-registered", middleware.JWTMiddleware(apiConfig.HandlerGetUserRegisteredContests))
	// create
	v1.Post("/contests", middleware.JWTMiddleware(apiConfig.HandlerCreateContest))
	// update
	v1.Put("/contests/users", middleware.JWTMiddleware(apiConfig.HandlerSetUsersInContest))
	v1.Put("/contests/problems", middleware.JWTMiddleware(apiConfig.HandlerSetProblemsInContest))
	v1.Put("/contests", middleware.JWTMiddleware(apiConfig.HandlerUpdateContest))
	// delete
	v1.Delete("/contests", middleware.JWTMiddleware(apiConfig.HanlderDeleteContest))

	// tournaments
	// search
	v1.Get("/tournaments", middleware.JWTMiddleware(apiConfig.HandlerGetTournament))
	v1.Get("/tournaments/rounds", middleware.JWTMiddleware(apiConfig.HandlerGetTournamentRound))
	v1.Post("/tournaments/search", middleware.JWTMiddleware(apiConfig.HandlerGetTournamentsByFilters))
	// create
	v1.Post("/tournaments", middleware.JWTMiddleware(apiConfig.HandlerCreateTournament))
	v1.Post("/tournaments/rounds", middleware.JWTMiddleware(apiConfig.HandlerCreateTournamentRound))
	// update
	v1.Put("/tournaments/contests", middleware.JWTMiddleware(apiConfig.HandlerChangeTournamentContest))
	return v1
}
