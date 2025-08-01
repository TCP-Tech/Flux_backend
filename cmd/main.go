package main

import (
	"database/sql"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/tcp_snm/flux/internal/api"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/email"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/auth_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/user_service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

var (
	apiConfig *api.Api
)

func initDatabase() *database.Queries {
	// get the database url
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		panic("dbURL not found")
	}

	// create a conneciton to the database
	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
		panic("cannot create a connection to db with url")
	}
	log.Info("Connection created to the database")

	// get the query tool with this connection
	return database.New(conn)
}

func initUserService(db *database.Queries) *user_service.UserService {
	log.Info("initializing user service")
	return &user_service.UserService{
		DB: db,
	}
}

func initAuthService(db *database.Queries, us *user_service.UserService) *auth_service.AuthService {
	log.Info("initializing auth service")
	return &auth_service.AuthService{
		DB:         db,
		UserConfig: us,
	}
}

func initProblemService(db *database.Queries, us *user_service.UserService) *problem_service.ProblemService {
	log.Info("initializing problem service")
	return &problem_service.ProblemService{
		DB:                db,
		UserServiceConfig: us,
	}
}

func initApi(db *database.Queries) *api.Api {
	log.Info("initializing api config")
	us := initUserService(db)
	log.Info("user service created")
	as := initAuthService(db, us)
	log.Info("auth service created")
	ps := initProblemService(db, us)
	log.Info("problem service created")
	a := api.Api{
		AuthServiceConfig:    as,
		ProblemServiceConfig: ps,
	}
	return &a
}

func setup() {
	godotenv.Load()
	service.InitializeServices()
	db := initDatabase()
	apiConfig = initApi(db)
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
