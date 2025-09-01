package user_service

import (
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
)

var (
	errMsgs = map[string]map[string]string{}
)

const (
	RoleManager   = "role_manager"
	RoleHC        = "role_hc"
	cacheCapacity = 50
)

type UserService struct {
	DB         *database.Queries
	rolesCache *lru.Cache[uuid.UUID, []string]
}

func (u *UserService) IntializeUserServices() error {
	log.Infof("intializing uuid->[]string (rolesCache) cache with capacity %d", cacheCapacity)
	cache, err := lru.New[uuid.UUID, []string](cacheCapacity)
	if err != nil {
		return err
	}
	u.rolesCache = cache
	return nil
}

type GetUsersRequest struct {
	UserIDs    []uuid.UUID `json:"user_ids"`
	UserNames  []string    `json:"user_names"`
	RollNos    []string    `json:"roll_nos"`
	PageNumber int32       `json:"page_number" validate:"min=1,max=10000"`
	PageSize   int32       `json:"page_size" validate:"min=0,max=10000"`
}

// used for profile viewing and auth stuff
type User struct {
	ID           uuid.UUID `json:"id"`
	RollNo       string    `json:"roll_no"`
	UserName     string    `json:"user_name"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
}

// used as a dto between different layers
type UserMetaData struct {
	UserID   uuid.UUID `json:"user_id"`
	UserName string    `json:"user_name"`
	RollNo   string    `json:"roll_no"`
}
