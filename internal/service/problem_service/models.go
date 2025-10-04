package problem_service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/lock_service"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

var (
	msgForeignKey = map[string]string{
		"fk_problem": "problem with given id doesn't exist",
	}

	msgUniqueConstraint = map[string]string{
		"uq_problem_id":      "entry with given problem id already exist",
		"uq_site_problem_code": "entry with given site_problem_code already exist",
	}

	errMsgs = map[string]map[string]string{
		flux_errors.CodeForeignKeyConstraint: msgForeignKey,
		flux_errors.CodeUniqueConstraint:     msgUniqueConstraint,
	}
)

const (
	EvalCodeforces                                  = "codeforces"
	EvalNil                                         = "invalid_evaluator"
	InternalProblemQuery service.InternalContextKey = "internal_problem_query"
)

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
	ID         int32      `json:"id"`
	Title      string     `json:"title" validate:"min=4,max=100"`
	Difficulty int32      `json:"difficulty" validate:"min=800,max=3000"`
	Evaluator  string     `json:"evaluator" validate:"oneof=codeforces"`
	LockID     *uuid.UUID `json:"lock_id"`
	CreatedBy  uuid.UUID  `json:"created_by"`

	LockTimeout *time.Time `json:"-"`
	LockAccess  *string    `json:"-"`
}

type StandardProblemData struct {
	ProblemID           int32             `json:"problem_id"`
	Statement           string            `json:"statement" validate:"required"`
	InputFormat         string            `json:"input_format" validate:"required"`
	OutputFormat        string            `json:"output_format" validate:"required"`
	FunctionDefinitions map[string]string `json:"function_definitions"`
	ExampleTestCases    *ExampleTestCases `json:"example_test_cases"`
	Notes               *string           `json:"notes"`
	MemoryLimitKB       int32             `json:"memory_limit_kb" validate:"min=1024"`
	TimeLimitMS         int32             `json:"time_limit_ms" validate:"min=500"`
	// if we built an evaluator that evaluates problem without submitting to any platform
	// then problemID is enough to identify the problem
	SiteProblemCode *string `json:"site_problem_code"`
}

type dbSpdParams struct {
	FunctionDefinitions *json.RawMessage
	ExampleTestCases    *json.RawMessage
}

// dto for requesting problems with fitlers
type GetProblemsRequest struct {
	Title           *string    `json:"title"`
	ProblemIDs      []int32    `json:"problem_ids"`
	LockID          *uuid.UUID `json:"lock_id"`
	Evaluator       *string    `json:"evaluator"`
	PageNumber      int32      `json:"page_number" validate:"numeric,min=1"`
	PageSize        int32      `json:"page_size" validate:"numeric,min=0,max=10000"`
	CreatorUserName *string    `json:"creator_user_name"`
	CreatorRollNo   *string    `json:"creator_roll_number"`
}
