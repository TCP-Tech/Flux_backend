package problem_service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type Platform string

type ProblemService struct {
	DB                *database.Queries
	UserServiceConfig *user_service.UserService
	LockServiceConfig *lock_service.LockService
}

type ExampleTestCase struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type ExampleTestCases struct {
	NumTestCases *int              `json:"num_test_cases"`
	Examples     []ExampleTestCase `json:"examples"`
}

type Problem struct {
	ID             int32             `json:"id"`
	Title          string            `json:"title" validate:"required,max=100"`
	Statement      string            `json:"statement" validate:"required"`
	InputFormat    string            `json:"input_format" validate:"required"`
	OutputFormat   string            `json:"output_format" validate:"required"`
	ExampleTCs     *ExampleTestCases `json:"example_test_cases"`
	Notes          *string           `json:"notes"`
	MemoryLimitKb  int32             `json:"memory_limit_kb" validate:"required,min=1024"`
	TimeLimitMs    int32             `json:"time_limit_ms" validate:"required,min=500"`
	Difficulty     int32             `json:"difficulty" validate:"required,min=800,max=3000"`
	SubmissionLink *string           `json:"submission_link" validate:"url"`
	Platform       *Platform         `json:"platform" validate:"omitempty,oneof=codeforces"`
	CreatedBy      uuid.UUID         `json:"created_by"`
	LastUpdatedBy  uuid.UUID         `json:"last_updated_by"`
	LockId         *uuid.UUID        `json:"lock_id"`
}

// helper struct for converting service problem data to db problem data
type dbProblemData struct {
	exampleTestCases *json.RawMessage
	platformType     database.NullPlatform
}

// helper struct for converting db problem data to service problem data
type serviceProblemData struct {
	exampleTestCases *ExampleTestCases
	platformType     *Platform
}

// dto for requesting problems with fitlers
type GetProblemsRequest struct {
	// title substring that might be in problem title
	Title string `json:"title"`
	// problems with certain ids
	ProblemIDs []int32 `json:"problem_ids"`
	// problems associated with a lock
	LockID *uuid.UUID `json:"lock_id"`
	// page number
	PageNumber int32 `json:"page_number" validate:"numeric,min=1"`
	// size of each page
	PageSize int32 `json:"page_size" validate:"numeric,min=0,max=10000"`
	// filter for the problem created
	CreatorUserName string `json:"creator_user_name"`
	CreatorRollNo   string `json:"creator_roll_number"`
}

// dto problems are requested based on filters
type ProblemMetaData struct {
	ProblemId  int32     `json:"problem_id"`
	Title      string    `json:"title"`
	Difficulty int32     `json:"difficulty"`
	Platform   *Platform `json:"platform"`
	CreatedBy  uuid.UUID `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`

	// field used only for internal purpose
	LockID      *uuid.UUID             `json:"-"`
	LockTimeout *time.Time             `json:"-"`
	LockAccess  *user_service.UserRole `json:"-"`
}
