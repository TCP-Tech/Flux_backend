package submission_service

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (slave *nyxSlave) recieveMail(mail mail) {
	slave.mailBox.Add(mail)
}

func (slave *nyxSlave) processMails() {
	for {
		// get the top mail
		topMail, ok := slave.mailBox.Pop()
		if !ok {
			time.Sleep(IdleMailBoxSleepTime)
			continue
		}

		// check the source
		if topMail.from != mailNyxMaster {
			slave.logger.Warnf(
				"recieved a mail=%v from unknown sender. mail ignored",
				topMail,
			)
			continue
		}

		// process
		switch body := topMail.body.(type) {
		case cfSub:
			// submit
			stat, err := slave.submitCfSolution(
				body.cfSubRequest,
				body.bot,
			)

			// inform
			result := cfSubResult{stat, err, body.submissionID}
			var priority = prNyxMstSubSuccess
			if err != nil {
				priority = prNyxMstSubFailed
			}
			slave.postman.postMail(mail{
				from:     slave.mailID,
				to:       mailNyxMaster,
				body:     result,
				priority: int(priority),
			})
		default:
			slave.logger.Errorf(
				"ignored mail with unknown body: %v",
				body,
			)
		}
	}
}

// TODO: complete this function
func (slave *nyxSlave) submitCfSolution(
	req cfSubRequest,
	bot Bot,
) (cfSubStatus, error) {
	// get a custom logger
	subLogger := slave.logger.WithFields(
		logrus.Fields{
			"sub_req_id": req.submissionID,
			"bot_name":   bot.Name,
		},
	)

	// prepare the json
	solutionMap := make(map[string]string)
	solutionMap["solution"] = req.solution
	solutionMap["language"] = req.language
	solutionMap["problem_index"] = req.problemIndex
	solutionMap["platform"] = platformCodeforces
	solutionMap["bot_cookies"] = bot.Cookies
	solutionMap["bot_name"] = bot.Name

	// marshal it
	solutionBytes, err := json.Marshal(solutionMap)
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot marshal %v, %w",
			flux_errors.ErrInternal,
			solutionMap,
			err,
		)
		subLogger.Error(err)
		return cfSubStatus{}, err
	}

	// dial a connection to the script
	conn, err := net.Dial("tcp", slave.scriptAddress)
	if err != nil {
		err = flux_errors.HandleIPCError(err)
		subLogger.Errorf(
			"failed to dial to script with address %s",
			slave.scriptAddress,
		)
		return cfSubStatus{}, err
	}

	// always close the connection in any case
	defer conn.Close()

	// get previous submission for the bot
	prevSub, err := slave.getLatestBotSubmission(bot.Name, 1)
	if err != nil {
		subLogger.Error("cannot submit solution. failed to get latest submission of bot")
		return cfSubStatus{}, err
	}

	// set a write deadline to avoid indefinite wait
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// request the script for evaluation
	err = writeToConn(
		conn,
		// append an extra line at the end so that json message can easily be decoded
		append(solutionBytes, '\n'),
	)
	if err != nil {
		subLogger.Error("cannot query the script for submission")
		return cfSubStatus{}, err
	}

	// set read deadline
	conn.SetReadDeadline(time.Now().Add(time.Second * 20))

	// read the response
	dec := json.NewDecoder(conn)
	var msg struct {
		Error string `json:"error"`
	}
	if err = dec.Decode(&msg); err != nil {
		err = flux_errors.HandleIPCError(err)
		subLogger.Error("cannot read response from script while submitting")
		return cfSubStatus{}, err
	}

	// check if it was submitted successfully
	if msg.Error != "" {
		err = fmt.Errorf(
			"%w, %s",
			flux_errors.ErrSubmissionFailed,
			msg.Error,
		)
		subLogger.Error("script encountered error during submission")
		return cfSubStatus{}, err
	}

	// get latest submission and confirm its not the previous submission
	var curSub cfSubStatus
	for i := 3; i > 0; i-- {
		curSub, err = slave.getLatestBotSubmission(bot.Name, 2)
		if err != nil {
			subLogger.Errorf("failed to get latest submission status of bot. submission wasted")
			return cfSubStatus{}, err
		}
		if curSub.id != prevSub.id {
			break
		}
		subLogger.Warnf(
			"latest submission is same as previous submission. trying %v more times",
			i-1,
		)
		time.Sleep(time.Second)
	}
	if curSub.id != prevSub.id {
		err = fmt.Errorf(
			"%w, latest submission id was same as previous submission id",
			flux_errors.ErrSubmissionFailed,
		)
		subLogger.Error(err)
		return cfSubStatus{}, err
	}

	subLogger.Debugf("request was successfully processed")

	return curSub, nil
}

func (slave *nyxSlave) getLatestBotSubmission(botName string, numExtraTries int) (cfSubStatus, error) {
	stat, err := slave.botManger.getLatestBotSubmission(botName)
	if err == nil {
		return stat, nil
	}

	slave.logger.Errorf(
		"failed to get latest submission of bot %v. trying %v more times",
		botName,
		numExtraTries,
	)

	for range numExtraTries {
		time.Sleep(time.Second * 1)
		stat, err := slave.botManger.getLatestBotSubmission(botName)
		if err == nil {
			return stat, nil
		}
	}

	err = fmt.Errorf(
		"%w, failed to get latest submission of bot %v after %v times",
		flux_errors.ErrInternal,
		botName,
		1+numExtraTries,
	)
	slave.logger.Error(err)
	return cfSubStatus{}, err
}
