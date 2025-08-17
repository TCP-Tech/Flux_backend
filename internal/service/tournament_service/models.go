package tournament_service

import (
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

var dbConstraintMessages = map[string]string{
	"fk_rounds_tournament": "Tournament does not exist.",
}

type TournamentService struct {
	DB                   *database.Queries
	ContestServiceConfig *contest_service.ContestService
	UserServiceConfig    *user_service.UserService
	LockServiceConfig    *lock_service.LockService
}

type Tournament struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title" validate:"min=5,max=100"`
	CreatedBy   uuid.UUID `json:"created_by"`
	IsPublished bool      `json:"is_published"`
	Rounds      int32     `json:"rounds"`
}

type TournamentRound struct {
	ID           uuid.UUID  `json:"-"`
	TournamentID uuid.UUID  `json:"tournament_id"`
	Title        string     `json:"title" validate:"min=5,max=100"`
	RoundNumber  int32      `json:"round_no"`
	LockID       *uuid.UUID `json:"lock_id"`
	CreatedBy    uuid.UUID  `json:"created_by"`

	// fields used internally
	LockAccess *user_service.UserRole `json:"-"`
	// currently this field is nil-only
	LockTimeout *time.Time `json:"-"`
}

type ChangeTournamentContestsRequest struct {
	TournamentID uuid.UUID   `json:"tournament_id"`
	RoundNumber  int32       `json:"round_no"`
	ContestIDs   []uuid.UUID `json:"contest_ids"`
}

type GetTournamentRequest struct {
	Title       string `json:"title"`
	IsPublished *bool  `json:"is_published"`
	PageSize    int32  `json:"page_size" validate:"min=0,max=10000"`
	PageNumber  int32  `json:"page_number" validate:"min=1,max=10000"`
}
