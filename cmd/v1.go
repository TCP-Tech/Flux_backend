package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/tcp_snm/flux/middleware"
)

func NewV1Router() *chi.Mux {
	v1 := chi.NewRouter()

	// configure all endpoints
	v1.Get("/healthz", middleware.JWTMiddleware(apiConfig.HandlerReadiness))
	v1.Post("/auth/signup", apiConfig.HandlerSignUp)
	v1.Post("/auth/login", apiConfig.HandlerLogin)
	v1.Get("/auth/signup", apiConfig.HandlerSignUpSendMail)
	v1.Get("/auth/reset-password", apiConfig.HandlerResetPasswordSendMail)
	v1.Post("/auth/reset-password", apiConfig.HandlerResetPassword)
	v1.Post("/problems", middleware.JWTMiddleware(apiConfig.HandlerAddProblem))
	v1.Put("/problems", middleware.JWTMiddleware(apiConfig.HandlerUpdateProblem))
	v1.Get("/problems", middleware.JWTMiddleware(apiConfig.HandlerGetProblemById))
	v1.Post("/problems/search", middleware.JWTMiddleware(apiConfig.HandlerGetProblemsByFilters))
	v1.Post("/locks", middleware.JWTMiddleware(apiConfig.HandlerCreateLock))
	v1.Get("/locks", middleware.JWTMiddleware(apiConfig.HandlerGetLocksByFilter))
	v1.Put("/locks", middleware.JWTMiddleware(apiConfig.HandlerUpdateLock))
	v1.Delete("/locks", middleware.JWTMiddleware(apiConfig.HanlderDeleteLockById))
	return v1
}
