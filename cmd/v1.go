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

	return v1
}
