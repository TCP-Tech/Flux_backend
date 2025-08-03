package problem_service

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type ProblemService struct {
	DB                *database.Queries
	UserServiceConfig *user_service.UserService
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
	MemoryLimitKB  int32             `json:"memory_limit_kb" validate:"required,numeric"`
	TimeLimitMS    int32             `json:"time_limit_ms" validate:"required,numeric"`
	Difficulty     int32             `json:"difficulty" validate:"required,numeric,min=800,max=3000"`
	SubmissionLink *string           `json:"submission_link" valdidate:"url"`
	Platform       *string           `json:"platform"`
	CreatedBy      uuid.UUID         `json:"created_by"`
	LastUpdatedBy  uuid.UUID         `json:"last_updated_by"`
	LockId         uuid.NullUUID     `json:"lock_id"`
}

type DBProblemData struct {
	exampleTestCases pqtype.NullRawMessage
	notes            sql.NullString
	submissionLink   sql.NullString
	platform         database.NullPlatformType
	lockId           uuid.NullUUID
}
