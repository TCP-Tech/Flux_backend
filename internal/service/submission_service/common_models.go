package submission_service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/contest_service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

var (
	// used for conversion of db error codes to user understandable messages
	errMsgs = map[string]map[string]string{}
	// PriorityQueue accepts only int as priority. But unix of a timestamp exceeds 32 bit.
	// this will cause trouble on 32 bit systems. so use this as a base to subtract from any
	// time stamp that is used as a comparable
	baseTimeStamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

const (
	IdleMailBoxSleepTime                               = time.Millisecond * 100
	internalSubmissionQuery service.InternalContextKey = "internal-subission-query"
)

type mailID string

// used by masters to send workers to sleep
type sleep time.Duration

const (
	mailScheduler         mailID = "mail@scheduler"
	mailSubmissionService mailID = "mail@submission_service"
	mailPostman           mailID = "mail@postman"
)

const (
	SubStatusFluxQueued = "flux_queued"
	SubStatusFluxFailed = "flux_failed"
)

var (
	nonSinkFluxStates = []string{SubStatusFluxFailed, SubStatusFluxQueued}
)

const (
	languageJava   = "java"
	languageCpp    = "cpp"
	languagePython = "python"
)

type SubmissionService struct {
	DB             *database.Queries
	ProblemService *problem_service.ProblemService
	ContestService *contest_service.ContestService
	Postman        *postman
	EvaluatorMails map[string]Evaluator
}

type SubmissionRequest struct {
	ProblemID int32             `json:"problem_id"`
	ContestID *uuid.UUID        `json:"contest_id"`
	Solution  map[string]string `json:"solution"`
}

type Evaluator interface {
	mailClient
	getSubmissionMailPriority() int
}

type subStatManager interface {
	getSubmission(context.Context, uuid.UUID) (any, error)
	updateSubmission(context.Context, *database.Queries, uuid.UUID, string) (fluxSubmission, error)
	bulkUpdateSubmissionState(context.Context, *database.Queries, []uuid.UUID, []string) error
}

type subStatManagerImpl struct {
	problemServiceConfig *problem_service.ProblemService
	contestServiceConfig *contest_service.ContestService
	db                   *database.Queries
	logger               *logrus.Entry
}

type mail struct {
	from     mailID
	to       mailID
	body     any
	priority int
}

type mailClient interface {
	recieveMail(mail mail)
	getMailID() mailID
}

// used by postman to inform the sender that the mail client is no
// longer valid. (might have been unregistered by someone else)
type invalidMailClient mailID

type unregisterMailClient mailID

type postman struct {
	sync.Mutex
	mailBox     chan mail
	mailClients map[mailID]mailClient
	logger      *logrus.Entry
}

type Prioritizable interface {
	GetPriority() int
}

type PriorityQueue[T Prioritizable] struct {
	inner *lane.PQueue
}

type fluxSubmission struct {
	SubmissionID uuid.UUID         `json:"submission_id"`
	SubmittedBy  uuid.UUID         `json:"submitted_by"`
	ProblemID    int32             `json:"problem_id"`
	ContestID    *uuid.UUID        `json:"contest_id"`
	Solution     map[string]string `json:"solution,omitempty"`
	State        string            `json:"submission_state"`
	SubmitedAt   time.Time         `json:"submitted_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// used as a generic sentinel signal to signal components to stop
type stop struct{}

// used as a generic sentinel signal to signam components to initiate a submission
type submit struct{}
