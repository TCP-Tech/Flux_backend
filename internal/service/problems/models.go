package problem_service

import (
	"github.com/tcp_snm/flux/internal/database"
	flux_user "github.com/tcp_snm/flux/internal/service/user"
)

type ProblemService struct {
	DB         *database.Queries
	UserConfig *flux_user.UserService
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
	ID             string            `json:"problem_id"`
	Title          string            `json:"title" validate:"required,max=25"`
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
}
