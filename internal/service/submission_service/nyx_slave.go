package submission_service

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (slave *nyxSlave) start() {
	// create a logger
	slave.logger = logrus.WithFields(
		logrus.Fields{
			"from": slave.mailID,
		},
	)

	// perform validations
	if slave.mailID == "" {
		panic("slave's mail id cant'be empty")
	}
	if slave.postman == nil {
		panic("slave expects non-nil postman")
	}
	if slave.botMgr == nil {
		panic("slave expects non-nil bot manager")
	}
	if slave.scriptAddress == "" {
		panic("slave expects non-empty script socket address")
	}

	// initialize only if its nil to preserve the old mails if it has panicked
	if slave.mailBox == nil {
		slave.mailBox = NewPriorityQueue[mail](lane.MAXPQ)
	}
	go slave.processMails()

	slave.logger.Info("slave started processing mails")
}

func (slave *nyxSlave) recieveMail(mail mail) {
	slave.mailBox.Add(mail)
}

func (slave *nyxSlave) processMails() {
	// inform the master that the slave has stoped processing
	// mails. useful if slave encounters error
	defer func() {
		if r := recover(); r != nil {
			slave.logger.Errorf("slave encountered unexpected error: %v", r)
			deadMail := mail{
				from:     slave.mailID,
				to:       mailNyxMaster,
				body:     slaveFailed(r),
				priority: prNyxMstSlvDead,
			}
			slave.postman.postMail(deadMail)
		}
	}()

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
		case cfSubRequest:
			shortSubID := getShortUUID(body.submissionID, 5)

			// get a bot from botventory
			bot, err := slave.botMgr.getBot(slave.taskID)
			if err != nil {
				slave.logger.Errorf(
					"slave failed to get a bot. aborting submission %v",
					shortSubID,
				)

				// inform master that slave failed to get a bot
				slave.postman.postMail(mail{
					from:     slave.mailID,
					to:       mailNyxMaster,
					body:     slaveBotError(err),
					priority: prNyxMstSlvBotErr,
				})
				slave.logger.Infof("informed master about slave bot error")

				// also inform that submission failed
				slave.postman.postMail(mail{
					from:     slave.mailID,
					to:       mailNyxMaster,
					body:     cfSubResult{err: err, submissionID: body.submissionID},
					priority: prNyxMstSubFailed,
				})
				slave.logger.Infof(
					"informed master that sub %v failed due to slave bot error",
					shortSubID,
				)

				continue
			}

			// submit
			stat, err := slave.submitCfSolution(body, bot)

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
		case stop:
			slave.logger.Warnf("recieved stop signal")
			slave.handleStopSignal()
			return
		case sleep:
			slave.logger.Infof(
				"slave recieved sleep command for %v seconds",
				time.Duration(body).Seconds(),
			)
			time.Sleep(time.Duration(body))
		default:
			slave.logger.Errorf(
				"ignored mail with unknown body: %v",
				body,
			)
		}
	}
}

// rejects all the requests
func (slave *nyxSlave) handleStopSignal() {
	for top, ok := slave.mailBox.Pop(); ok; top, ok = slave.mailBox.Pop() {
		switch body := top.body.(type) {
		case cfSubRequest:
			failMail := mail{
				from: slave.mailID,
				to:   mailNyxMaster,
				body: cfSubResult{
					err: fmt.Errorf("%w, slave recieved stop signal", flux_errors.ErrInternal),
				},
			}
			slave.postman.postMail(failMail)
			slave.logger.Debugf(
				"informed master about failure of submission %v after recieving stop signal",
				getShortUUID(body.submissionID, 5),
			)
		case stop:
			slave.logger.Warnf("recieved multiple stop signals")
		case sleep:
			slave.logger.Warnf(
				"ignoring sleep command for %v seconds",
				time.Duration(body).Seconds(),
			)
		default:
			slave.logger.Errorf("cannot process mail %v while handling stop signal", top)
		}
	}
	slave.logger.Info("handled stop signal")
}

func (slave *nyxSlave) submitCfSolution(
	req cfSubRequest,
	bot Bot,
) (cfSubStatus, error) {
	// record time for sampling submission time
	startTime := time.Now()

	// get a custom logger
	subLogger := slave.logger.WithFields(
		logrus.Fields{
			"sub_req_id": getShortUUID(req.submissionID, 5),
			"bot_name":   bot.Name,
		},
	)

	// get the file extension
	fileExtension, err := getFileExtensionFromLanguage(req.language)
	if err != nil {
		err = fmt.Errorf(
			"%w, failed to get file extension from language %v, %w",
			flux_errors.ErrInternal,
			req.language,
			err,
		)
		subLogger.Error(err)
		return cfSubStatus{}, err
	}

	// create a solution file
	solutionFilePath, err := createRandomFile(
		"/tmp",
		req.submissionID.String(),
		fileExtension,
		3,
	)
	if err != nil {
		slave.logger.Error(
			"%w, cannot create solution file. submission failed",
			err,
		)
		return cfSubStatus{}, err
	}

	// add a random comment at the start of the file to bypass duplicate solutions
	randomComment, err := getRandomComment(req.language)
	if err != nil {
		subLogger.Errorf(
			"failed to generate random comment for language %v",
			req.language,
		)
		return cfSubStatus{}, err
	}

	// prepare the solution bytes
	solutionBytes := append([]byte(randomComment), '\n')
	solutionBytes = append(solutionBytes, []byte(req.solution)...)

	// write solution in the file
	err = os.WriteFile(solutionFilePath, solutionBytes, 0644)
	if err != nil {
		err = fmt.Errorf(
			"%w, failed to write solution to file %s, %w",
			flux_errors.ErrInternal,
			solutionFilePath,
			err,
		)
		subLogger.Error(err)
		return cfSubStatus{}, err
	}

	// delete the solution file at last
	defer func() {
		err := os.Remove(solutionFilePath)
		if err != nil {
			err = fmt.Errorf(
				"cannot delete solution file %v, %w",
				solutionFilePath,
				err,
			)
			subLogger.Error(err)
		}
	}()

	// prepare the solution
	solutionMap := make(map[string]any)
	solutionMap["cookies"] = bot.Cookies
	solutionMap["language"] = req.language
	solutionMap["solution_file_path"] = solutionFilePath
	solutionMap["bot_name"] = bot.Name
	solutionMap["site_problem_code"] = req.siteProblemCode
	solutionMap["submission_id"] = req.submissionID

	// prepare the request
	socketReq := make(map[string]any)
	socketReq["req_type"] = "submit"
	socketReq["platform"] = platformCodeforces
	socketReq["solution"] = solutionMap

	// marshal it
	requestBytes, err := json.Marshal(socketReq)
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
		err = flux_errors.WrapIPCError(err)
		subLogger.Errorf(
			"failed to dial to script with address %s",
			slave.scriptAddress,
		)
		return cfSubStatus{}, err
	}

	// always close the connection in any case
	defer conn.Close()

	// get previous submission for the bot
	prevSub, err := slave.botMgr.getLatestBotSubmission(bot.Name)
	if err != nil {
		subLogger.Error("cannot submit solution. failed to get latest submission of bot")
		return cfSubStatus{}, err
	}
	subLogger.Debugf("previous cf submission id of bot %v: %v", bot.Name, prevSub.CfSubID)

	// set a write deadline to avoid indefinite wait
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// request the script for evaluation
	err = writeToConn(
		conn,
		// append an extra line at the end so that json message can easily be decoded
		append(requestBytes, '\n'),
	)
	if err != nil {
		subLogger.Errorf("%v, cannot query the script for submission", err)
		return cfSubStatus{}, err
	}
	subLogger.Debugf("sent socket request to the script for submission to %v", platformCodeforces)

	// set read deadline
	conn.SetReadDeadline(time.Now().Add(time.Second * 90))

	// read the response
	dec := json.NewDecoder(conn)
	var msg struct {
		Error     string            `json:"error"`
		UserError bool              `json:"user_error"`
		Cookies   map[string]string `json:"cookies"`
	}
	if err = dec.Decode(&msg); err != nil {
		err = flux_errors.WrapIPCError(err)
		subLogger.Errorf("%v, cannot read response from script while submitting", err)
		return cfSubStatus{}, err
	}

	// update cookies irrespective of result
	if len(msg.Cookies) > 0 {
		// ask bot manager to update the cookies
		if err = slave.botMgr.updateBotCookies(bot.Name, msg.Cookies); err != nil {
			subLogger.Errorf(
				"bot manager failed to update cookies of bot %v",
				bot.Name,
			)
		} else {
			subLogger.Debugf("updated cookies of bot %v", bot.Name)
		}
	} else {
		subLogger.Warnf("no cookies were returned by script of bot %v", bot.Name)
	}

	// check if it was submitted successfully
	if msg.Error != "" {
		// check if its a bot error
		if msg.Error == "bot" {
			// inform master that the bot became corrupted
			slave.postman.postMail(mail{
				from:     slave.mailID,
				to:       mailNyxMaster,
				body:     corruptedBot(bot.Name),
				priority: prNyxMstSlvBotCorrupted,
			})

			slave.logger.Infof("informed master that bot %v is corrupted", bot.Name)
		}
		if msg.UserError {
			// TODO: complete this

		}
		err = fmt.Errorf(
			"%w, %s",
			flux_errors.ErrSubmissionFailed,
			msg.Error,
		)
		subLogger.Error("script encountered error during submission")
		return cfSubStatus{}, err
	}

	// get latest submission and confirm its not the previous submission
	// since script didn't encounter any error, there is a high probability that the submission is
	// successful. So, attempt multiple tries to get the latest submission
	var curSub cfSubStatus
	for i := 3; i > 0; i-- {
		curSub, err = slave.botMgr.getLatestBotSubmission(bot.Name)
		if err != nil {
			subLogger.Errorf("failed to get latest submission status of bot. retrying %v more times", i-1)
			continue
		}
		if curSub.CfSubID != prevSub.CfSubID {
			break
		}
		subLogger.Warnf(
			"latest submission is same as previous submission. trying %v more times",
			i-1,
		)
		time.Sleep(time.Second * 5)
	}
	if err != nil || curSub.CfSubID <= prevSub.CfSubID {
		err = fmt.Errorf(
			"%w, latest submission id was same as previous submission id after multiple queries",
			flux_errors.ErrSubmissionFailed,
		)
		subLogger.Error(err)
		return cfSubStatus{}, err
	}
	subLogger.Debugf("recieved latest submission: %v", curSub)

	subLogger.Debugf("request was successfully processed")

	// report monitor about submission time
	slave.postman.postMail(
		mail{
			from: slave.mailID,
			to:   mailNyxLdMnr,
			body: subTAlert{
				duration:   time.Since(startTime),
				sampleTime: time.Now(),
			},
		},
	)

	return curSub, nil
}

// concurrent safe because mailID will never gonna change once assigned
func (slave *nyxSlave) getMailID() mailID {
	return slave.mailID
}
