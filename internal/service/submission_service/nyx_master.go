package submission_service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/scheduler_service"
)

func (master *nyxMaster) Start(
	db *database.Queries,
	cfQueryUrl string,
	subStatMgr subStatManager,
) {
	// initialize logger
	master.logger = logrus.WithFields(logrus.Fields{
		"from": mailNyxMaster,
	})

	// perform validations
	if master.postman == nil {
		panic("master expects non-nil postman")
	}
	if master.scrStCmd.Name == "" {
		panic("script start command should not be blank")
	}
	if len(master.bots) == 0 {
		panic("non-0 number of bots must be passed to initialize nyx master")
	}
	if master.scheduler == nil {
		panic("master expects non-nil scheduler")
	}
	if subStatMgr == nil {
		panic("master expectx non-nil submission status manager")
	}

	// setup load manager
	loadManager := nyxLdMnr{
		postman: master.postman,
	}
	// register with postman
	master.postman.RegisterMailClient(mailNyxLdMnr, &loadManager)
	loadManager.start()

	// start bot manager
	mgr := nyxBotMgr{
		cfQueryUrl: cfQueryUrl,
		DB:         db,
		postman:    master.postman,
		subStatMgr: subStatMgr,
	}
	mgr.start(master.bots)
	master.botMgr = &mgr

	// setup botventory
	master.botventory = &nyxBotventory{}
	master.botventory.start()

	master.mailBox = (NewPriorityQueue[mail](lane.MAXPQ))
	master.slaves = make([]*nyxSlave, 0)
	master.lastUsedSlave = -1 // to represent that we haven't used any slave yet
	master.activeSubmissions = make(map[uuid.UUID]nyxActSub)
	master.pendingSlaves = make(map[uuid.UUID]pendingSlave)
	master.killedSlaves = make([]*nyxSlave, 0)

	if err := master.startSlave(80); err != nil {
		panic(err)
	}

	go master.processMails()

	master.logger.Info("master started processing mails")
}

func (master *nyxMaster) processMails() {
	for {
		topMail, ok := master.mailBox.Pop()
		if !ok {
			time.Sleep(IdleMailBoxSleepTime)
			continue
		}

		switch topMail.body.(type) {
		case cfSubRequest:
			master.handleCfSubRequest(topMail)
		case cfSubResult:
			master.handleCfSubResult(topMail)
		case slvTaskLnhContainer:
			master.handleSlvTaskLaunch(topMail)
		case slaveReady:
			master.handleSlaveReady(topMail)
		case slvScrDead:
			master.handleDeadSlave(topMail)
		case loadReport:
			master.handleLoadReport(topMail)
		case slaveBotError:
			master.handleSlaveBotError(topMail)
		case invalidMailClient:

		case slaveFailed:
			master.handleFailedSlave(topMail)
		}

	}
}

func (master *nyxMaster) handleSlaveBotError(botMail mail) {
	botErr := error(botMail.body.(slaveBotError))

	if errors.Is(botErr, flux_errors.ErrNotFound) {
		slv := master.getSlaveByMailID(botMail.from)
		if slv != nil {
			master.logger.Warnf(
				"slave %v is in master's inventory but not in botventory. redistributing bots",
				botMail.from,
			)
			master.botventory.redistributeBots(master.bots, master.getSlavesInfo())
			return
		}

		master.logger.Warnf(
			"slave %v is not in master's inventory but requested for bots. sending stop signal to it",
			botMail.from,
		)
		master.postman.postMail(mail{
			from:     mailNyxMaster,
			to:       botMail.from,
			body:     stop{},
			priority: prNyxSlvStop,
		})
		return
	}

	if errors.Is(botErr, errNoBots) {
		if len(master.bots) >= len(master.slaves) {
			master.logger.Warnf(
				"more bots than slaves but slave %v has no bots assigned. redistributing bots",
				botMail.from,
			)
			master.botventory.redistributeBots(master.bots, master.getSlavesInfo())
			return
		}
		master.logger.Warnf(
			"more slaves than bots. killing %v slaves and redistributing bots",
			len(master.slaves)-len(master.bots),
		)
		master.killLeastRecentlyUsedSlaves(len(master.slaves) - len(master.bots))
		master.botventory.redistributeBots(master.bots, master.getSlavesInfo())
		return
	}

	master.logger.Errorf(
		"slave %v encountered unknown error in getting bots. killing it for safety",
		botMail.from,
	)
	master.killSlaveWithID(botMail.from)
}

func (master *nyxMaster) handleFailedSlave(failMail mail) {
	// check if the slave is present in the inventory
	slave := master.getSlaveByMailID(failMail.from)

	if slave == nil {
		master.logger.Warnf("slave %v not found in invenotry but sent failure", failMail.from)
		return
	}

	// restart it
	slave.start()
	master.logger.Warnf("restarted slave %v after it sent failure", slave.mailID)
}

func (master *nyxMaster) handleCfSubRequest(reqMail mail) {
	body := reqMail.body.(cfSubRequest)

	// used for logging purpose
	shortSubID := getShortUUID(body.submissionID, 5)

	// check if there is an active entry of the submission
	actSub, active := master.activeSubmissions[body.submissionID]
	if active {
		// 1. check if its corresponding slave exist
		// 2. even though slave exist, we cannot prove that the submission is in progress.
		//    the slave is built to report every submission status after evalution.
		//    but in rare unknown cases (when there is an unknown bug exist) it might go unreported
		slaveExist := master.slaveExist(actSub.slaveID)
		if slaveExist {
			master.logger.Warnf(
				"ignoring duplicate request for submission with id %v",
				shortSubID,
			)
			return
		}
		master.logger.Warnf(
			"active entry of submission with id %v found but corresponding slave not found",
			shortSubID,
		)
	}

	// inform the load manager about the request
	alertMail := mail{
		from:     mailNyxMaster,
		to:       mailNyxLdMnr,
		body:     subAlert(time.Now()),
		priority: prNyxSlvSubReq,
	}
	master.postman.postMail(alertMail)

	// check if there are any slaves available
	if len(master.slaves) == 0 {
		err := fmt.Errorf(
			"%w, no slave found to process request",
			flux_errors.ErrInternal,
		)
		master.logger.Error(err)

		// inform the watcher
		master.postman.postMail(
			mail{
				from:     mailNyxMaster,
				to:       reqMail.from,
				body:     cfSubResult{err: err},
				priority: prNyxWtSubFailed,
			},
		)

		return
	}

	// slaves are used in round robin manner
	nextSlaveIdx := (master.lastUsedSlave + 1) % len(master.slaves)
	nextSlave := master.slaves[nextSlaveIdx]
	master.lastUsedSlave = nextSlaveIdx

	// create an active sub entry
	actSub = nyxActSub{
		from:         reqMail.from,
		submissionID: body.submissionID,
		slaveID:      nextSlave.taskID,
	}
	master.activeSubmissions[body.submissionID] = actSub
	master.logger.Debugf(
		"created an active sub entry for sub %v",
		shortSubID,
	)

	// inform the slave about the submission
	master.postman.postMail(
		mail{
			from:     mailNyxMaster,
			to:       nextSlave.mailID,
			body:     body,
			priority: prNyxSlvSubReq,
		},
	)
	master.logger.Debugf(
		"asked slave %v to process sub req %v",
		nextSlave.mailID,
		shortSubID,
	)
}

func (master *nyxMaster) handleCfSubResult(res mail) {
	// cast body
	body := res.body.(cfSubResult)

	// get the active submission entry
	actSub, ok := master.activeSubmissions[body.submissionID]
	if !ok {
		master.logger.WithField("result", body).Errorf(
			"active submission entry for submission request was not found",
		)
		return
	}

	// decide priority
	var mailPriority int
	if body.err == nil {
		mailPriority = prNyxWtSubSuccess
	} else {
		mailPriority = prNyxWtSubFailed
	}

	// inform the watcher
	master.postman.postMail(mail{
		from:     mailNyxMaster,
		to:       actSub.from,
		body:     body,
		priority: mailPriority,
	})
	master.logger.Debugf(
		"request %v was processed by slave. informed watcher",
		getShortUUID(body.submissionID, 5),
	)

	// delete its entry
	delete(master.activeSubmissions, actSub.submissionID)
}

func (master *nyxMaster) handleSlvTaskLaunch(ml mail) {
	// cast body to the task_response
	resp := scheduler_service.TaskResponse(ml.body.(slvTaskLnhContainer))

	// get the pending launch
	pdSlv, ok := master.pendingSlaves[resp.TaskID]
	if !ok {
		// probably slave might have died just after starting and
		// it might have been handled by master already
		master.logger.Warnf("pending slave with task id %v was not found in the map", resp.TaskID)
		return
	}

	master.logger.Infof(
		"slave with task id %v have been launched",
		getShortUUID(resp.TaskID, 5),
	)

	// create a context for waiting
	ctx, cancel := context.WithCancel(context.Background())
	pdSlv.waitForScriptCancel = cancel

	// update
	master.pendingSlaves[resp.TaskID] = pdSlv

	// wait until the script accepts connections
	go master.waitForNyxScrReady(ctx, pdSlv)
}

// periodically reads the socket address file for address where the script listens through.
func (master *nyxMaster) waitForNyxScrReady(ctx context.Context, pdSlv pendingSlave) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Total timeout for the whole operation
	timeout := time.After(60 * time.Second)

	// used for logging
	shortTaskID := getShortUUID(pdSlv.taskID, 5)

	for {
		select {
		case <-ctx.Done():
			master.logger.Infof(
				"context was done while getting pending slave %v ready",
				shortTaskID,
			)
			return
		case <-timeout:
			// failed to read the socket address. inform the mail processor
			err := fmt.Errorf(
				"%w, slave failed to write address into its socket file",
				flux_errors.ErrComponentStart,
			)
			master.logger.Error(err)
			ready := slaveReady{pendingSlave: pdSlv, err: err}
			master.postman.postMail(mail{
				from:     mailNyxMaster,
				to:       mailNyxMaster,
				body:     ready,
				priority: prNyxMstScrDead,
			})
			return
		case <-ticker.C:
			// aquire os level file lock to avoid corrupted reads
			pdSlv.sockLock.RLock()

			// read from file
			data, err := os.ReadFile(pdSlv.sockAddFile)
			// unlock right after reading cause we only care about the latest content of the file
			pdSlv.sockLock.Unlock()
			if err != nil {
				err = fmt.Errorf(
					"%w, error occurred while reading socket address file (%v) of slave %v",
					flux_errors.ErrComponentStart,
					pdSlv.sockAddFile,
					shortTaskID,
				)
				master.logger.Error(err)

				ready := slaveReady{pendingSlave: pdSlv, err: err}

				errMail := mail{
					from:     mailNyxMaster,
					to:       mailNyxMaster,
					body:     ready,
					priority: prNyxMstScrDead,
				}
				master.postman.postMail(errMail)
				return
			}

			addStr := string(data)
			if addStr == "" {
				continue
			}

			// read address
			add := strings.Split(addStr, " ")
			if len(add) != 2 {
				master.logger.Warnf(
					"script written its address but number of words are %v",
					len(add),
				)
				continue
			}

			sockAdd := fmt.Sprintf("%v:%v", add[0], add[1])
			master.logger.Debugf(
				"script of slave with task id %v started listening at address: %v",
				shortTaskID, sockAdd,
			)

			readyMail := mail{
				from: mailNyxMaster,
				to:   mailNyxMaster,
				body: slaveReady{
					add:          sockAdd,
					err:          nil,
					pendingSlave: pdSlv,
				},
				priority: prNyxMstScrLaunched,
			}

			// inform
			master.postman.postMail(readyMail)
			return
		}
	}
}

func (master *nyxMaster) handleSlaveReady(ml mail) {
	// cast body to slaveReady
	ready := ml.body.(slaveReady)

	// cancel the context to release resources
	ready.waitForScriptCancel()

	// check if the slave is still pending (may be deleted if it has been terminated already)
	_, ok := master.pendingSlaves[ready.taskID]
	if !ok {
		master.logger.Warnf(
			"cannot handle slave ready. there is no pending slave with task id %v",
			ready.taskID,
		)
		return
	}

	// check for errors
	if ready.err != nil {
		master.logger.Warnf(
			"slave with task id %v failed to get ready",
			ready.taskID,
		)
		return
		// all the cleanup will be done only by dead slave function
	}

	// construct new slave
	var slMailId mailID = mailID("mail@slave_" + getShortUUID(ready.taskID, 5))
	newSlave := nyxSlave{
		pendingSlave:  ready.pendingSlave,
		postman:       master.postman,
		mailID:        slMailId,
		scriptAddress: ready.add,
		botMgr:        master.botMgr,
		botventory:    master.botventory,
	}

	// register with postman
	if err := master.postman.RegisterMailClient(slMailId, &newSlave); err != nil {
		master.logger.Errorf(
			"postman failed to register slave %v. killing slave's script",
			slMailId,
		)
		// if the scheduler fails to kill the task, there is nothing we can do with the error,
		// as this error occur very rarely. If it does, then we need to reconsider our whole design
		if err = master.scheduler.KillTask(newSlave.taskID); err != nil {
			master.logger.Errorf("scheduler failed to kill slave %v", slMailId)
		}
		return
	}

	newSlave.start()

	master.slaves = append(master.slaves, &newSlave)
	master.logger.Infof("new slave (%v) has been added to inventory", newSlave.mailID)

	// remove pending slave
	delete(master.pendingSlaves, newSlave.taskID)

	// redistribute bots
	master.botventory.redistributeBots(master.bots, master.getSlavesInfo())
}

func (master *nyxMaster) handleDeadSlave(ml mail) {
	// cast body
	deadSlaveInfo := ml.body.(slvScrDead)

	// get the slave
	var deadSlave *nyxSlave
	for _, slv := range master.slaves {
		if slv.taskID == deadSlaveInfo.taskID {
			deadSlave = slv
			break
		}
	}
	// check once in killed slaves
	if deadSlave == nil {
		killedSlaves := make([]*nyxSlave, 0)
		for _, killedSlave := range master.killedSlaves {
			if deadSlaveInfo.taskID == killedSlave.taskID {
				deadSlave = killedSlave
			} else {
				killedSlaves = append(killedSlaves, killedSlave)
			}
		}
		master.killedSlaves = killedSlaves
	}
	if deadSlave == nil {
		// slave might have died just after start and might not have been handled by master yet
		pdSlv, ok := master.pendingSlaves[deadSlaveInfo.taskID]
		if !ok {
			master.logger.Warnf(
				"recieved a dead mail for slave with id %v which isn't present in slave inventory and in pending slaves",
				getShortUUID(deadSlaveInfo.taskID, 5),
			)
			return
		}

		// stop the waitForScriptReady goroutine
		if pdSlv.waitForScriptCancel != nil {
			pdSlv.waitForScriptCancel()
		}

		// delete the files its holding
		master.deleteSlaveFiles(pdSlv.sockAddFile, pdSlv.sockLock.Path())

		// remove the pending slave
		delete(master.pendingSlaves, pdSlv.taskID)

		master.logger.Warnf("deleted pending slave with task id %v", pdSlv.taskID)
		return
	}

	// stop slave
	master.postman.postMail(
		mail{
			from:     mailNyxMaster,
			to:       deadSlave.mailID,
			body:     stop{},
			priority: prNyxSlvStop,
		},
	)
	master.logger.Infof("signaled slave with id %v to stop", deadSlave.mailID)

	// unregister the slave from postman
	master.postman.postMail(mail{
		from: mailNyxMaster,
		to:   mailPostman,
		body: unregisterMailClient(deadSlave.mailID),
	})

	// delete files
	master.deleteSlaveFiles(deadSlave.sockAddFile, deadSlave.sockLock.Path())

	// delete slave
	newSlaves := make([]*nyxSlave, 0, len(master.slaves)-1)
	for _, slvCont := range master.slaves {
		if slvCont.taskID == deadSlave.taskID {
			continue
		}
		newSlaves = append(newSlaves, slvCont)
	}
	master.slaves = newSlaves

	// redistribute bots
	master.logger.Infof("redistributing bots after handling dead slave with id %v", deadSlave.taskID)
	master.botventory.redistributeBots(master.bots, master.getSlavesInfo())
}

func (master *nyxMaster) getSlavesInfo() []slaveInfo {
	res := make([]slaveInfo, 0, len(master.slaves))
	for _, slv := range master.slaves {
		res = append(res, slaveInfo{slv.taskID, slv.mailID})
	}
	return res
}

func (master *nyxMaster) deleteSlaveFiles(sockAddFile string, sockLockFile string) {
	for _, filePath := range []string{sockAddFile, sockLockFile} {
		if err := os.Remove(filePath); err != nil {
			err = fmt.Errorf(
				"cannot delete file \"%s\" of slave: %w",
				filePath,
				err,
			)
			master.logger.Error(err)
		}
	}
}

func (master *nyxMaster) getOnSlaveDead() func(uuid.UUID, error) {
	return func(taskID uuid.UUID, err error) {
		deadMail := mail{
			from: mailNyxMaster,
			to:   mailNyxMaster,
			body: slvScrDead{
				taskID: taskID,
				err:    err,
			},
			priority: prNyxMstScrDead,
		}

		master.postman.postMail(deadMail)
	}
}

func (master *nyxMaster) handleLoadReport(loadMail mail) {
	report := loadMail.body.(loadReport)

	// calculate recommended number of slaves for smooth submissions
	// calculate biased ceil ie., if recommended slaves >= x.4 then we will conisder it as x+1 slaves
	recSlaves := int(0.6 + report.avgLoad*report.avgSubT.Minutes())

	// if requests are too high, then max number of slaves will be ceil(num_bots)/2
	capSlaves := min((len(master.bots)+1)/2, recSlaves)

	// case where we should launch more slaves. we should have atleast 1 slave up at all times
	totalSlaves := len(master.slaves) + len(master.pendingSlaves)
	minSlaves := max(1, capSlaves)
	if totalSlaves < minSlaves {
		// set a base priority as 30
		master.logger.Infof(
			"current slaves (active + pending): %v, min slaves required for current load: %v, launching more slaves",
			totalSlaves,
			minSlaves,
		)
		currentPriority := max(30, 80-10*totalSlaves)
		for range minSlaves - totalSlaves {
			master.startSlave(int32(currentPriority))
			currentPriority = max(currentPriority-10, 30)
		}
	}

	// if we have more slaves, we wont consider pendingSlaves as it disrupt consistency
	// however, they may be taken into consideration during further load report evaluations
	if len(master.slaves) > capSlaves {
		// if we have only one slave greater than recommended slaves we ignore it as noise
		if len(master.slaves)-1 == capSlaves {
			master.logger.Infof(
				"recommended slaves: %v, current slaves: %v, ignoring recommendation",
				capSlaves,
				len(master.slaves),
			)
			return
		}

		master.logger.Infof(
			"current active slaves: %v, recommended slaves: %v, killing some slaves",
			len(master.slaves),
			capSlaves,
		)

		// we kill least recently used slaves
		master.killLeastRecentlyUsedSlaves(len(master.slaves) - capSlaves)
	}

}

func (master *nyxMaster) killLeastRecentlyUsedSlaves(num int) {
	if num > len(master.slaves) {
		master.logger.Warnf(
			"asked to kill %v slaves. but only %v slave are there in inventory",
			num, len(master.slaves),
		)
		num = len(master.slaves)
	}

	omittedSlaves := make([]mailID, 0)
	for i := 1; i <= num; i++ {
		slaveIdx := (master.lastUsedSlave + i) % len(master.slaves)
		slave := master.slaves[slaveIdx]

		// send a stop signal to the slave
		master.postman.postMail(mail{
			from:     mailNyxMaster,
			to:       slave.mailID,
			body:     stop{},
			priority: prNyxSlvStop,
		})
		master.logger.Debugf("sent stop signal to slave %v", slave.mailID)

		if err := master.scheduler.KillTask(slave.taskID); err != nil {
			master.logger.Errorf(
				"failed to kill slave %v",
				slave.mailID,
			)
			continue
		}
		master.logger.Debugf(
			"requested scheduler to kill slave %v",
			slave.mailID,
		)
		omittedSlaves = append(omittedSlaves, slave.mailID)
	}

	// omit slaves
	master.omitSlaves(omittedSlaves...)
}

func (master *nyxMaster) killSlaveWithID(slaveID mailID) {
	slave := master.getSlaveByMailID(slaveID)
	if slave == nil {
		master.logger.Errorf(
			"slave %v was asked to kill but absent in inventory",
			slaveID,
		)
		return
	}

	// send a stop signal to slave
	master.postman.postMail(mail{
		from:     mailNyxMaster,
		to:       slaveID,
		body:     stop{},
		priority: prNyxSlvStop,
	})
	master.logger.Debugf("sent stop signal to slave %v", slaveID)

	if err := master.scheduler.KillTask(slave.taskID); err != nil {
		master.logger.Errorf(
			"failed to kill slave %v",
			slave.mailID,
		)
		return
	}
	master.logger.Debugf("asked scheduler to kill slave %v", slaveID)

	master.omitSlaves(slaveID)
}

func (master *nyxMaster) omitSlaves(slaveIDs ...mailID) {
	// Put the IDs to omit into a set for fast lookups
	omitSet := make(map[mailID]struct{})
	for _, id := range slaveIDs {
		omitSet[id] = struct{}{}
	}

	finalSlaves := make([]*nyxSlave, 0, len(master.slaves))
	for _, slv := range master.slaves {
		if _, found := omitSet[slv.mailID]; found {
			// This is a slave to be killed, move it to the killedSlaves list
			master.killedSlaves = append(master.killedSlaves, slv)
		} else {
			// This slave is being kept, add it to the new active list
			finalSlaves = append(finalSlaves, slv)
		}
	}

	master.slaves = finalSlaves
}

func (master *nyxMaster) recieveMail(mail mail) {
	master.mailBox.Add(mail)
}

func (master *nyxMaster) slaveExist(slaveID uuid.UUID) bool {
	for _, slave := range master.slaves {
		if slave.taskID == slaveID {
			return true
		}
	}
	return false
}

// Generate a text file for the python script to write the socket through which
// it listens for requests. Then ask the scheduler for launching the python script
// along with passing the socket address file as an argument in the command.
func (master *nyxMaster) startSlave(priority int32) error {
	sockAddFile, err := createRandomFile("/tmp", "slave", "txt", 10)
	if err != nil {
		master.logger.Errorf(
			"%v, failed to create socket address file. cannot start a slave", err,
		)
		return err
	}
	sockLock := flock.New(sockAddFile + ".lock")

	// arguments being passed to the command
	cmdArgs := slices.Clone(master.scrStCmd.ExtraArgs)
	cmdArgs = append(cmdArgs, "-f", sockAddFile)

	// construct the request
	slaveTaskRequest := scheduler_service.TaskRequest{
		Name: "nyx slave",
		// TODO: review this
		Resources: scheduler_service.Resources{
			CPU:    180,
			Memory: 800,
		},
		Command: scheduler_service.Command{
			Name:        master.scrStCmd.Name,
			Args:        cmdArgs,
			CmdExecType: scheduler_service.CmdLongRunning,
		},
		Priority:          priority,
		SchedulingRetries: 2,
		OnLaunchComplete:  master.getOnSlaveTaskLaunch(),
		OnTaskComplete:    master.getOnSlaveDead(),
	}

	// request scheduler to lanch
	taskID, err := master.scheduler.ScheduleTask(slaveTaskRequest)
	if err != nil {
		return err
	}
	master.logger.Infof("new slave with task id %v has been requested to schedule", taskID)

	pendingSlave := pendingSlave{
		taskID:      taskID,
		sockAddFile: sockAddFile,
		sockLock:    sockLock,
	}
	master.pendingSlaves[taskID] = pendingSlave

	return nil
}

func (master *nyxMaster) getSlaveByMailID(id mailID) *nyxSlave {
	for _, slv := range master.slaves {
		if slv.mailID == id {
			return slv
		}
	}
	return nil
}

func (master *nyxMaster) getMailID() mailID {
	return mailNyxMaster
}

// effectively called by scheduler
func (master *nyxMaster) getOnSlaveTaskLaunch() func(scheduler_service.TaskResponse) {
	return func(tr scheduler_service.TaskResponse) {
		launchMail := mail{
			from:     mailScheduler,
			to:       mailNyxMaster,
			body:     slvTaskLnhContainer(tr),
			priority: prNyxMstScrLaunched,
		}
		master.postman.postMail(launchMail)
	}
}
