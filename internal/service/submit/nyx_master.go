package submission_service

import (
	"errors"
	"fmt"
	"time"

	"github.com/tcp_snm/flux/internal/flux_errors"
)

// TODO: complete this
func (master *nyxMaster) Start(postman *postman) error {
	return errors.New("master not yet initialized")
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
			go master.loadManager.subAlert(time.Now().UTC())
			master.handleCfSubRequest(topMail)
		case cfSubResult:
			master.handleCfSubResult(topMail)
		}

	}
}

func (master *nyxMaster) handleCfSubRequest(reqMail mail) error {
	body := reqMail.body.(cfSubRequest)

	// check if there is an active entry of the submission
	actSub, active := master.activeSubmissions[body.submissionID]
	if active {
		// 1. check if its corresponding slave exist
		// 2. even though slave exist, we cannot prove that the submission is in progress.
		//    the slave is built to report every submission status after evalution.
		//    but in rare unknown cases (when there is an unknown bug exist) it might go unreported
		slaveExist := master.slaveExist(actSub.slaveID)
		if slaveExist {
			master.logger.Warnf("ignoring duplicate request for submission with id %v", actSub.submissionID)
			return nil
		}
		master.logger.Warnf(
			"active entry of submission with id %v found but corresponding slave not found",
			actSub.submissionID,
		)
	}

	// ensure there are non-zero slaves
	if len(master.slaves) == 0 {
		err := fmt.Errorf(
			"%w, no nyx slaves found for submission of req with id %v",
			flux_errors.ErrInternal,
			body.submissionID,
		)
		master.logger.Warn(err)
		return err
	}

	// slaves are used in round robin manner
	slaveIdx := max(0, (master.lastUsedSlave+1)%len(master.slaves))
	master.lastUsedSlave = slaveIdx
	slave := master.slaves[slaveIdx]
	master.logger.Debugf(
		"using slave with id %v for submission with id %v",
		slave.slave.mailID,
		body.submissionID,
	)

	// ensure slave has bots
	if len(slave.bots) == 0 {
		err := fmt.Errorf(
			"%w, no bots found for slave with id %v",
			flux_errors.ErrInternal,
			slave.slave.mailID,
		)
		master.logger.Error(err)
		return err
	}

	// decide the bot to use for the slave
	botIdx := max(0, (1+slave.lastUsedBot)%len(slave.bots))
	bot := slave.bots[botIdx]
	slave.lastUsedBot = botIdx
	master.logger.Debugf("using bot %v for sumbission with id %v", bot, body.submissionID)

	// create an active entry
	actSub = nyxActSub{
		from:         reqMail.from,
		submissionID: body.submissionID,
		slaveID:      slave.slave.mailID,
		botName:      bot.Name,
	}
	master.activeSubmissions[body.submissionID] = actSub

	// construct the mail
	sub := cfSub{
		cfSubRequest: body,
		bot:          bot,
	}
	cfSubMail := mail{
		from: mailNyxMaster,
		to:   slave.slave.mailID,
		body: sub,
	}

	// send the mail to the slave
	master.postman.postMail(cfSubMail)

	return nil
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
	if body.err != nil {
		mailPriority = prNyxWtSubSuccess
	} else {
		mailPriority = prNyxWtSubFailed
	}

	// construct result mail
	resMail := mail{
		from:     mailNyxMaster,
		to:       actSub.from,
		body:     body,
		priority: mailPriority,
	}

	// post
	master.postman.postMail(resMail)
}

func (master *nyxMaster) recieveMail(mail mail) {
	master.mailBox.Add(mail)
}

func (master *nyxMaster) slaveExist(slaveID mailID) bool {
	for _, slave := range master.slaves {
		if slave.slave.mailID == slaveID {
			return true
		}
	}
	return false
}
