package submission_service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (mgr *nyxManager) Start(dbSubPollSeconds int64) {
	if mgr.postman == nil {
		panic("nyx manager expects non-nil postman")
	}
	if mgr.db == nil {
		panic("nyx manager expects non-nil db")
	}
	if mgr.subQrr == nil {
		panic("manager expects non-nil submission querier")
	}

	if mgr.probSerConfig == nil {
		panic("manager expects non-nil problem service config")
	}

	if mgr.mailBox == nil {
		mgr.mailBox = NewPriorityQueue[mail](lane.MAXPQ)
	}

	mgr.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailNyxManager,
		},
	)

	mgr.watchers = make(map[uuid.UUID]*nyxWatcher)

	// launch a submission poller
	cnclCtx, cancel := context.WithCancel(context.Background())
	ctx := context.WithValue(cnclCtx, internalSubmissionQuery, struct{}{})
	go mgr.pollPendingSubmissionsFromDb(ctx, dbSubPollSeconds)

	go mgr.processMails(cancel)
	mgr.logger.Info("nyx submission manager started processing mails")
}

func (mgr *nyxManager) processMails(pollSubCnclFunc context.CancelFunc) {
	defer pollSubCnclFunc()
	for {
		topMail, ok := mgr.mailBox.Pop()
		if !ok {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		switch topMail.body.(type) {
		case fluxSubmission:
			mgr.handleSubmission(topMail)
		case invalidMailClient:
			mgr.handleInvalidMailClient(topMail)
		case cfSubResult:
			mgr.handleCfSubResult(topMail)
		case watcherFailed:
			mgr.handleFailedWatcher(topMail)
		default:
			mgr.logger.Errorf("ignoring invalid mail %v", topMail)
		}
	}
}

func (mgr *nyxManager) handleFailedWatcher(failMail mail) {
	// cast the body
	watFailed := failMail.body.(watcherFailed)

	// used for logging purpose
	shortSubID := getShortUUID(watFailed.submissionID, 5)

	// get the watcher
	wt, ok := mgr.watchers[watFailed.submissionID]
	if !ok {
		mgr.logger.Warnf(
			"watcher of sub %v is not in inventory but sent failure",
			shortSubID,
		)
		return
	}

	// restart the watcher
	if err := wt.start(); err != nil {
		mgr.logger.Errorf(
			"failed to restart watcher of sub %v after it encountered failure",
			shortSubID,
		)
		return
	}
	mgr.logger.Debugf(
		"restarted watcher of sub %v after it sent failure",
		shortSubID,
	)
}

func (mgr *nyxManager) handleCfSubResult(wtMail mail) {
	res := wtMail.body.(cfSubResult)
	if res.err != nil {
		mgr.logger.Errorf(
			"watcher %v encountered error during watch. submission aborted",
			wtMail.from,
		)
	}

	// check if corresponding watcher is present
	wt, ok := mgr.watchers[res.submissionID]
	if !ok {
		mgr.logger.Warnf(
			"recieved cf submission result of submission %v but corresponding watcher is not found",
			res.submissionID,
		)
		return
	}

	// delete the watcher
	delete(mgr.watchers, res.submissionID)
	mgr.logger.Debugf(
		"recieved cf submission result. watcher %v removed from inventory",
		wtMail.from,
	)

	// unregister with postman
	mgr.postman.postMail(mail{
		from: mailNyxManager,
		to:   mailPostman,
		body: unregisterMailClient(wt.mailID),
	})
	mgr.logger.Debugf("sent mail to postman to unregister watcher %v", wt.mailID)
}

func (mgr *nyxManager) handleInvalidMailClient(invMail mail) {
	invMailID := mailID(invMail.body.(invalidMailClient))
	var wt *nyxWatcher
	for _, v := range mgr.watchers {
		if v.mailID == invMailID {
			wt = v
			break
		}
	}
	if wt == nil {
		mgr.logger.Warnf(
			"postman reported watcher %v as invalid but is absent in inventory",
			invMailID,
		)
		return
	}

	// register with postman
	if err := mgr.postman.RegisterMailClient(invMailID, wt); err != nil {
		mgr.logger.Errorf(
			"failed to register watcher %v that's reported as invalid by postman but present in inventory",
			invMailID,
		)
	}
}

func (mgr *nyxManager) handleSubmission(subMail mail) {
	// cast the body
	fluxSub := subMail.body.(fluxSubmission)

	// used for logging purpose
	shortSubID := getShortUUID(fluxSub.SubmissionID, 5)

	// check if the watcher for that submission is already present
	_, ok := mgr.watchers[fluxSub.SubmissionID]
	if !ok {
		mgr.logger.Debugf(
			"watcher for submission %v is not present. creating a new one",
			shortSubID,
		)
		if err := mgr.createWatcher(fluxSub); err != nil {
			return
		}
	}

	// send a mail to watcher to subit
	mgr.postman.postMail(
		mail{
			from:     mailNyxManager,
			to:       mgr.watchers[fluxSub.SubmissionID].mailID,
			body:     submit{},
			priority: prNyxWtSub,
		},
	)
	mgr.logger.Debugf("asked watcher %v to submit", mgr.watchers[fluxSub.SubmissionID].mailID)
}

// WARN: not adaptive to the load
func (mgr *nyxManager) pollPendingSubmissionsFromDb(ctx context.Context, pollSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(pollSeconds))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pendingSubs, err := mgr.db.PollPendingSubmissions(ctx, nonSinkFluxStates)
			if err != nil {
				err = flux_errors.HandleDBErrors(
					err,
					errMsgs,
					"failed to poll pending submissions from db by nyx manager",
				)
			}

			if len(pendingSubs) == 0 {
				break // break from select
			}

			for _, pdSub := range pendingSubs {
				fluxSub, err := dbSubmissionToFluxSubmission(pdSub)
				if err != nil {
					mgr.logger.Errorf(
						"failed to convert db submission with id %v to fluxSubmission",
						getShortUUID(pdSub.ID, 5),
					)
					continue
				}

				// alert manager
				mgr.postman.postMail(mail{
					from:     mailNyxManager,
					to:       mailNyxManager,
					body:     fluxSub,
					priority: prNyxMgrSubAlert,
				})

			}

			mgr.logger.Debugf("alerted manager about %v pending submissions", len(pendingSubs))
		case <-ctx.Done():
			mgr.logger.Warnf("context was done. no more polling of pending submissions from db")
		}
	}
}

func (mgr *nyxManager) createWatcher(fluxSub fluxSubmission) error {
	// NOTE: currently there is only codeforces platform. If the number increases
	// then get the problem from the problem_service and then pass the evaluator to watcher

	// create a random mail id and register with postman
	shortID := getShortUUID(fluxSub.SubmissionID, 8)
	wtMailID := mailID(fmt.Sprintf("mail@watcher-%s", shortID))

	// create a new watcher
	watcher := nyxWatcher{
		submissionID:  fluxSub.SubmissionID,
		platform:      platformCodeforces,
		postman:       mgr.postman,
		mailID:        wtMailID,
		solution:      fluxSub.Solution,
		DB:            mgr.db,
		subQrr:        mgr.subQrr,
		probSerConfig: mgr.probSerConfig,
	}

	// Register with the postman
	err := mgr.postman.RegisterMailClient(watcher.mailID, &watcher)
	if err != nil {
		err = fmt.Errorf(
			"%w, failed to register watcher for submission %v with mailID %v: %v",
			flux_errors.ErrInternal, fluxSub.SubmissionID, watcher.mailID, err,
		)
		mgr.logger.Error(err)
		return err
	}

	// register with manager
	mgr.watchers[fluxSub.SubmissionID] = &watcher

	// start the watcher
	if err := watcher.start(); err != nil {
		mgr.logger.Errorf("watcher %v failed to start. submission failed", watcher.mailID)
		return err
	}

	return nil
}

func (mgr nyxManager) getMailID() mailID {
	return mailNyxManager
}

func (mgr nyxManager) getSubmissionMailPriority() int {
	return prNyxMgrSubAlert
}

func (mgr *nyxManager) recieveMail(ml mail) {
	mgr.mailBox.Add(ml)
}
