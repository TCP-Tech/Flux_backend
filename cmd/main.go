package main

import (
	"context"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/tcp_snm/flux/internal/api"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/auth_service"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/tournament_service"
	"github.com/tcp_snm/flux/internal/service/user_service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

var (
	apiConfig *api.Api
)

func initDatabase() (*pgxpool.Pool, *database.Queries) {
	// get the database url
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		panic("dbURL not found")
	}

	// create a conneciton to the database
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		panic(err)
	}

	// get the query tool with this connection
	return pool, database.New(pool)
}

func initUserService(db *database.Queries) *user_service.UserService {
	log.Info("initializing user service")
	us := user_service.UserService{
		DB: db,
	}
	err := us.IntializeUserServices()
	if err != nil {
		panic(err)
	}
	return &us
}

func initAuthService(db *database.Queries, us *user_service.UserService) *auth_service.AuthService {
	log.Info("initializing auth service")
	return &auth_service.AuthService{
		DB:         db,
		UserConfig: us,
	}
}

func initLockService(db *database.Queries, us *user_service.UserService) *lock_service.LockService {
	return &lock_service.LockService{
		DB:                db,
		UserServiceConfig: us,
	}
}

func initProblemService(
	db *database.Queries,
	ls *lock_service.LockService,
	us *user_service.UserService,
) *problem_service.ProblemService {
	log.Info("initializing problem service")
	return &problem_service.ProblemService{
		DB:                db,
		LockServiceConfig: ls,
		UserServiceConfig: us,
	}
}

func initContestService(
	db *database.Queries,
	ls *lock_service.LockService,
	us *user_service.UserService,
	ps *problem_service.ProblemService,
) *contest_service.ContestService {
	log.Info("initializing contest service")
	return &contest_service.ContestService{
		DB:                   db,
		LockServiceConfig:    ls,
		UserServiceConfig:    us,
		ProblemServiceConfig: ps,
	}
}

func initTournamentService(
	db *database.Queries,
	us *user_service.UserService,
	ls *lock_service.LockService,
	cs *contest_service.ContestService,
) *tournament_service.TournamentService {
	log.Info("initializing tournament service")
	return &tournament_service.TournamentService{
		DB:                   db,
		UserServiceConfig:    us,
		LockServiceConfig:    ls,
		ContestServiceConfig: cs,
	}
}

func initApi(pool *pgxpool.Pool, db *database.Queries) *api.Api {
	log.Info("initializing api config")
	us := initUserService(db)
	log.Info("user service created")
	as := initAuthService(db, us)
	log.Info("auth service created")
	ls := initLockService(db, us)
	log.Info("lock service created")
	ps := initProblemService(db, ls, us)
	log.Info("problem service created")
	cs := initContestService(db, ls, us, ps)
	log.Info("contest service created")
	ts := initTournamentService(db, us, ls, cs)
	log.Info("tournament service created")
	a := api.Api{
		AuthServiceConfig:       as,
		ProblemServiceConfig:    ps,
		LockServiceConfig:       ls,
		ContestServiceConfig:    cs,
		TournamentServiceConfig: ts,
	}
	return &a
}

func setup() {
	godotenv.Load()
	log.SetFormatter(&log.TextFormatter{
		// Force colors to be enabled
		ForceColors: true,
		// Add the full timestamp
		FullTimestamp: true,
	})
	pool, db := initDatabase()
	service.InitializeServices(pool)
	apiConfig = initApi(pool, db)
	email.StartEmailWorkers(1)
}

func setCors(router *chi.Mux) {
	router.Use(
		cors.Handler(
			cors.Options{
				AllowedOrigins:   []string{"https://*", "http://*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"*"},
				AllowCredentials: false,
				ExposedHeaders:   []string{"Link"},
				MaxAge:           300,
			},
		),
	)
	log.Info("cors options has been set")
}

func main() {
	setup()

	// initialize a new router
	router := chi.NewRouter()
	setCors(router)

	// mount v1 router
	v1router := NewV1Router()
	router.Mount("/v1", v1router)
	log.Info("v1 router has been mounted")

	// find port for the server to start
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Warnf("port not found in environment. using default port %s", port)
	}

	// find the address to start the server
	apiAddress := os.Getenv("API_URL") + ":" + port

	log.Info("starting server")
	// create a server object to listen to all requests
	srv := http.Server{
		Handler: router,
		Addr:    apiAddress,
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Server cannot be started. Error: %v", err)
		return
	}

}
