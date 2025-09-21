package submission_service

import (
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	platformCodeforces = "codeforces"
	reqLoadWindowSize  = 10
)

// used by master
const (
	prNyxMstSubReq = iota
	prNyxMstSubFailed
	prNyxMstSubSuccess
	prNyxMstScrTerminated
	prNyxMstScrLaunched
	prNyxMstLoadReport
)

// used by multiple clients
const (
	prNyxWtSubFailed = iota
	prNyxWtSubSuccess
)

const (
	mailNyxMaster mailID = "mail@nyx_script_master"
	mailCfBotMgr  mailID = "mail@cf_bot_manager"
	mailNyxLdMnr  mailID = "mail@nyx_load_monitor"
)

type subTimeStamp int

// the main Nyx evaluator registered in the registry of submission service
type EvalNyx struct {
	subReqQueue chan uuid.UUID
}

// data object to store details of a bot
type Bot struct {
	Name     string
	Platform string `validate:"oneof=codeforces"`
	Cookies  string
}

// slave will recieve from and write mails to the master only
// slave must use the every time its going to change its state except while recieving mail
// state of a slave is owned by it and no external entity including the master should change it directly
// all the communication to the slave must happen through the mails only
type nyxSlave struct {
	postman       *postman
	mailID        mailID
	scriptAddress string // python script's socket address
	mailBox       *PriorityQueue[mail]
	botManger     *cfBotManager
	logger        *logrus.Entry
}

// adapter around slave for master to keep track of slaves and their metadata
type nyxSlaveContainer struct {
	slave       *nyxSlave
	bots        []Bot
	lastUsedBot int
}

// monitors load frequently to adjust resouces
type nyxLdMnr struct {
	pq            *PriorityQueue[subTimeStamp]
	reqTimeStamps chan time.Time
	loadWindow    []int32
	logger        *logrus.Entry
	postman       *postman
}

type nyxMaster struct {
	mailBox           *PriorityQueue[mail]
	postman           *postman
	slaves            []nyxSlaveContainer // since slaves are 5-10 in number, slice is convinient to use
	lastUsedSlave     int
	Bots              map[string][]Bot        // platform (eg codeforces, leetcode) -> []Bot
	activeSubmissions map[uuid.UUID]nyxActSub // submissionID -> nyxWatcher
	logger            *logrus.Entry
	loadManager       *nyxLdMnr
}

// used by the master to keep track of all active submission requests from the watcher
type nyxActSub struct {
	from         mailID
	submissionID uuid.UUID
	slaveID      mailID
	botName      string
}

// generic struct used by multiple platforms like cf, leetcode etc
type nyxSubReq struct {
	from         mailID
	submissionID uuid.UUID
}

// used by watcher to request the master for cf submission
type cfSubRequest struct {
	nyxSubReq
	solution     string
	language     string
	problemIndex string
}

// used by master to ask the slave to submit to cf
type cfSub struct {
	cfSubRequest
	bot Bot
}

// used to convey the result of the submission
// usually from slave -> master -> watcher
type cfSubResult struct {
	status       cfSubStatus
	err          error
	submissionID uuid.UUID
}

// used by the bot monitor to keep track of active submissions of the
// bot in codeforces
type cfSubStatus struct {
	id                  int64
	verdict             string
	timeConsumedMillis  int
	memoryConsumedBytes int
	passedTestCount     int
}

// this will keep querying cf for latest submissions status of a bot
type cfBotMonitor struct {
	sync.Mutex
	botName   string // this is also the mailID of the monitor
	newSubs   chan cfSubStatus
	status    map[int64]cfSubStatus
	cfBaseUrl *url.URL
	postman   *postman
}

// manages multiple bot monitors
type cfBotManager struct {
	monitors  map[string]*cfBotMonitor // botName -> monitor
	cfBaseUrl string
	postman   *postman
}

// used by load manager to report master about recent load statistics
type reqLoadReport struct {
	loadWindow []int32
}
