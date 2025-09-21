package submission_service

import (
	"time"

	"github.com/google/uuid"
	"github.com/oleiade/lane"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

var (
	// used for conversion of db error codes to user understandable messages
	errMsgs = map[string]map[string]string{}
)

const (
	IdleMailBoxSleepTime = time.Millisecond * 100
)

type mailID string

const (
	SubStatusFluxQueued    = "flux_queued"
	SubStatusFluxSubmitted = "flux_submitted"
	SubStatusFluxRunning   = "flux_running"
)

type SubmissionService struct {
	DB             *database.Queries
	ProblemService *problem_service.ProblemService
	ContestService *contest_service.ContestService
	Evaluators     map[string]Evaluator
}

type SubmissionRequest struct {
	ProblemID int32             `json:"problem_id"`
	ContestID *uuid.UUID        `json:"contest_id"`
	Solution  map[string]string `json:"solution"`
}

type Evaluator interface {
	evaluate(submissionID uuid.UUID)
}

type mail struct {
	from     mailID
	to       mailID
	body     any
	priority int
}

type mailClient interface {
	recieveMail(mail mail)
}

type postman struct {
	mailBox     chan mail
	mailClients map[mailID]mailClient
}

type Comparable interface {
	GetPriority() int
}

type PriorityQueue[T Comparable] struct {
	inner *lane.PQueue
}
