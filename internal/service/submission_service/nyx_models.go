package submission_service

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/service/problem_service"
	"github.com/tcp_snm/flux/internal/service/scheduler_service"
)

// internal errors
var (
	errNoBots    = errors.New("no bots to assign")
	cfSinkStates = []string{
		"FAILED", "OK", "PARTIAL", "COMPILATION_ERROR", "RUNTIME_ERROR", "WRONG_ANSWER",
		"TIME_LIMIT_EXCEEDED", "MEMORY_LIMIT_EXCEEDED", "IDLENESS_LIMIT_EXCEEDED", "SECURITY_VIOLATED",
		"CRASHED", "INPUT_PREPARATION_CRASHED", "CHALLENGED", "SKIPPED", "REJECTED",
	}
)

const (
	platformCodeforces = "codeforces"
)

// used by master
const (
	prNyxMstSubReq = iota
	prNyxMstSubFailed
	prNyxMstSubSuccess
	prNyxMstLoadReport
	prNyxMstScrLaunched
	prNyxMstSlvBotErr
	prNyxMstSlvDead
	prNyxMstScrDead
)

const (
	prNyxMgrSubAlert = iota
	prNyxMgrSubResult
	prNyxMgrWtEnded
)

const (
	prNyxSlvSubReq = iota
	prNyxSlvStop
)

// used by multiple clients
const (
	prNyxWtSub = iota
	prNyxWtSubFailed
	prNyxWtSubSuccess
)

const (
	mailNyxMaster  mailID = "mail@nyx_script_master"
	mailNyxBotMgr  mailID = "mail@nyx_bot_manager"
	mailNyxLdMnr   mailID = "mail@nyx_load_monitor"
	mailNyxManager mailID = "mail@nyx_sub_manager"
)

// used by watcher for extracting values from solution map
const (
	KeySolution = "solution"
	KeyLanguage = "language"
)

type subTimeStamp int

type subAlert time.Time

type nyxManager struct {
	mailBox       *PriorityQueue[mail]
	db            *database.Queries
	logger        *logrus.Entry
	postman       *postman
	watchers      map[uuid.UUID]*nyxWatcher
	subQrr        subStatManager // used to initialize watcher
	probSerConfig *problem_service.ProblemService
}

// data object to store details of a bot
type Bot struct {
	Name     string
	Platform string `validate:"oneof=codeforces"`
	Cookies  map[string]string
}

// used to keep track of all the slaves that is currently being scheduled by the scheduler
type pendingSlave struct {
	taskID              uuid.UUID
	sockAddFile         string
	sockLock            *flock.Flock
	waitForScriptCancel context.CancelFunc
}

// slave is the owner of a nyx python script. all the communication from go app to python
// will be done via this slave. Slaves are owned by nyx master.
type nyxSlave struct {
	pendingSlave
	postman       *postman
	mailID        mailID
	scriptAddress string // python script's socket address
	mailBox       *PriorityQueue[mail]
	botMgr        *nyxBotMgr
	logger        *logrus.Entry
	botventory    *nyxBotventory
}

type subT struct {
	previousSampleTime time.Time
	previousAverage    time.Duration
}

// used by slave to report monitor about the submission time taken by the latest submission it has done
type subTAlert struct {
	duration   time.Duration
	sampleTime time.Time
}

// monitors load frequently to adjust resouces
type nyxLdMnr struct {
	sync.Mutex
	subReqT  *PriorityQueue[subTimeStamp]
	mailBox  chan mail
	avgLoad  float64 // latest samples of number of requests per minute
	avgSubT  subT    // latest samples of average submission time in minute per request
	subTChan chan subTAlert
	logger   *logrus.Entry
	postman  *postman
}

// used to report master that slave is actively accepting connections
type slaveReady struct {
	pendingSlave
	add string
	err error
}

// used to report master that scheduler has launched the slave task
type slvTaskLnhContainer scheduler_service.TaskResponse

type slvScrDead struct {
	taskID uuid.UUID
	err    error
}

type NyxScrStrtCmd struct {
	Name      string
	ExtraArgs []string
}

type nyxMaster struct {
	logger            *logrus.Entry
	postman           *postman
	scrStCmd          NyxScrStrtCmd // base command used start the python script
	bots              []Bot
	scheduler         *scheduler_service.Scheduler // used for scheduling slaves
	botMgr            *nyxBotMgr
	mailBox           *PriorityQueue[mail]
	slaves            []*nyxSlave             // since slaves are 5-10 in number, slice is convinient to use
	lastUsedSlave     int                     // helps in using slaves in round-robin manner
	activeSubmissions map[uuid.UUID]nyxActSub // submissionID -> nyxWatcher
	pendingSlaves     map[uuid.UUID]pendingSlave
	botventory        *nyxBotventory
	// used to keep track of slaves that have been scheduled to kill
	killedSlaves []*nyxSlave
}

// used by the master to keep track of all active submission requests from the watcher
type nyxActSub struct {
	from         mailID
	submissionID uuid.UUID
	slaveID      uuid.UUID
}

type nyxWatcher struct {
	submissionID  uuid.UUID
	platform      string
	postman       *postman
	mailBox       *PriorityQueue[mail]
	mailID        mailID
	logger        *logrus.Entry
	solution      map[string]string
	DB            *database.Queries
	subQrr        subStatManager
	probSerConfig *problem_service.ProblemService
}

type cfSubRequest struct {
	submissionID    uuid.UUID
	solution        string
	language        string
	siteProblemCode string
}

// used to convey the result of the submission along with some meta-data
type cfSubResult struct {
	status       cfSubStatus
	err          error
	submissionID uuid.UUID
}

// used for parsing json data queried from codeforces and also
// as a dto for transferring the results between different components
type cfSubStatus struct {
	CfSubID             int64  `json:"id"`
	Verdict             string `json:"verdict"`
	TimeConsumedMillis  int32  `json:"timeConsumedMillis"`
	MemoryConsumedBytes int32  `json:"memoryConsumedBytes"`
	PassedTestCount     int32  `json:"passedTestCount"`
}

func (stat *cfSubStatus) equal(other cfSubStatus) bool {
	return stat.CfSubID == other.CfSubID && stat.Verdict == other.Verdict &&
		stat.TimeConsumedMillis == other.TimeConsumedMillis &&
		stat.MemoryConsumedBytes == other.MemoryConsumedBytes && stat.PassedTestCount == other.PassedTestCount
}

// this is responsible for querying all the submission status made using the bot
// being monitored by this component. One monitor can only monitor one bot. If there are
// any changes from previous query from cf, it will then update those changes into db.
type cfBotMonitor struct {
	sync.Mutex
	botName    string // this is also the mailID of the monitor
	newSubs    chan cfSubStatus
	subStatMap map[int64]cfSubStatus
	cfQueryUrl *url.URL
	postman    *postman
	DB         *database.Queries // used for updating
	logger     *logrus.Entry
	subStatMgr subStatManager
}

// manages multiple bot monitors
type nyxBotMgr struct {
	monitors   map[string]*cfBotMonitor // botName -> monitor
	cfQueryUrl string
	DB         *database.Queries
	postman    *postman
	logger     *logrus.Entry
	subStatMgr subStatManager
}

// used by load manager to report master about recent load statistics
type loadReport struct {
	avgLoad float64
	// estimated submission time taken by slave to process a request
	// currently not platform specific
	avgSubT time.Duration
}

// status of a submission made to cf retrieved from db
// pointers are used to represent those fields are optional
// e.g., when their status has not yet been queried from cf
type dbCfSubStatus struct {
	fluxSubmission
	cfSubID             *int64 `json:"-"`
	TimeConsumedMillis  *int32 `json:"time_consumed_millis"`
	MemoryConsumedBytes *int32 `json:"memory_consumed_bytes"`
	PassedTestCount     *int32 `json:"passed_test_count"`
}

// used as a dto to transfer slave's metadata
type slaveInfo struct {
	slaveID   uuid.UUID // the task id of the slave's nyx script given by scheduler
	slvMailId mailID
}

// used by nyxBotventory for keeping track of slaves and their bots
type nyxSlaventory struct {
	slaveInfo
	bots        []Bot
	lastUsedBot int
}

// if the same bot is used by two different slaves then, we cannot
// map cf site submission id to flux submission id. Also, master must
// allocate bots fairly to each slave without changing their internal state directly.
// So this mutex component is used as a common memory between master and slaves
type nyxBotventory struct {
	sync.Mutex
	logger    *logrus.Entry
	inventory map[uuid.UUID]nyxSlaventory
}

// if a slave is not able to get a bot, it wraps the error into this and reports to master
type slaveBotError error

// if a slave fails due to unexpected reasons then recovers and informs master which
// then gracefully restart the slave if needed
type slaveFailed any

type watcherFailed struct {
	recoveredError any
	submissionID   uuid.UUID
}
