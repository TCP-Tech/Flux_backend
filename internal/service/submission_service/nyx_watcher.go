package submission_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
	"github.com/tcp_snm/flux/internal/service/problem_service"
)

func (wt *nyxWatcher) start() error {
	if wt.mailID == "" {
		panic("watcher mailID is empty")
	}

	// initialize logger
	wt.logger = logrus.WithFields(logrus.Fields{
		"from":          wt.mailID,
		"submission_id": getShortUUID(wt.submissionID, 5),
	})

	if wt.postman == nil {
		panic("watcher expects non-nil postman")
	}

	// NOTE: currently only codeforces is supported as platform. modify this when extended
	if wt.platform != platformCodeforces {
		panic("watcher initialized with invalid platform " + wt.platform)
	}

	// validate the solution
	for _, exp := range []string{KeySolution, KeyLanguage} {
		_, ok := wt.solution[exp]
		if !ok {
			err := fmt.Errorf(
				"%w, %s is not present in the solution map",
				flux_errors.ErrComponentStart,
				exp,
			)
			wt.logger.Error(err)
			return err
		}
	}

	if wt.mailBox == nil {
		wt.mailBox = NewPriorityQueue[mail](lane.MAXPQ)
	}

	if wt.DB == nil {
		panic("watcher expects non-nil db")
	}

	if wt.subQrr == nil {
		panic("watcher expects non-nil submission querier")
	}

	if wt.probSerConfig == nil {
		panic("watcher expects non-nil problem service config")
	}

	go wt.processMails()
	wt.logger.Debugf("watcher started processing mails")

	return nil
}

func (wt *nyxWatcher) processMails() {
	defer func() {
		if r := recover(); r != nil {
			wt.logger.Errorf("watcher encountered unexpected error: %v", r)
			deadMail := mail{
				from:     wt.mailID,
				to:       mailNyxMaster,
				body:     watcherFailed{r, wt.submissionID},
				priority: prNyxMstSlvDead,
			}
			wt.postman.postMail(deadMail)
		}
	}()

	for {
		topMail, ok := wt.mailBox.Pop()
		if !ok {
			time.Sleep(IdleMailBoxSleepTime)
			continue
		}

		// all the helper methods like cfSubmit should return one bool i.e., endWatch
		switch topMail.body.(type) {
		case submit:
			switch wt.platform {
			case platformCodeforces:
				if endWatch := wt.cfSubmit(); endWatch {
					return
				}
			default:
				wt.logger.Errorf("unknown platform of watcher: %v. cannot submit", wt.platform)
				return
			}
		case cfSubResult:
			if endWatch := wt.handleSubResult(topMail); endWatch {
				return
			}
		}
	}
}

func (wt *nyxWatcher) cfSubmit() bool {
	// get a context with timeout to avoid indefinite wait
	ctx, cancel := wt.getInternalQueryCtx(time.Second * 10)
	defer cancel()

	// set state to false if incase anything goes wrong
	subSuccess := false
	var subSuccessPtr *bool = &subSuccess

	// initially assume submission would go wrong. this function would be called just before returning
	// if everything goes right, we flip the flag at the end to avoid update. Otherwise, update state to failure
	defer func() {
		if *subSuccessPtr {
			return
		}
		wt.updateSubStateToFailure(ctx)
	}()

	// get submission from db
	subStat, err := wt.subQrr.getSubmission(ctx, wt.submissionID)
	if err != nil {
		wt.logger.Error("submission querier failed to get status of submission. cannot process submit request")
		return false
	}

	// cast subStat to cfSubStat
	cfSubStat, ok := subStat.(dbCfSubStatus)
	if !ok {
		// this wont change until application is restarted with changes. dont retry
		err := fmt.Errorf(
			"%w, cannot cast submission response from querier to dbCfSubStatus",
			flux_errors.ErrInternal,
		)
		wt.logger.Error(err)

		// inform the manager
		wt.informManagerAboutWatchEnd(cfSubResult{err: err, submissionID: wt.submissionID})

		// end watch
		return true
	}

	// check if the submission has already been evaluated
	if cfSubStat.cfSubID != nil {
		err := fmt.Errorf(
			"%w, cfsubId (%v) is not nil. aborting submission",
			flux_errors.ErrEntityAlreadyExist,
			cfSubStat.cfSubID,
		)
		wt.logger.Warn(err)

		// inform the manager
		wt.informManagerAboutWatchEnd(cfSubResult{err: err, submissionID: wt.submissionID})

		// end watch
		return true
	}

	// get the required fields from map
	solution := wt.solution[KeySolution]
	language := wt.solution[KeyLanguage]

	// get the problem from problem service config to get the submission url
	// we get this everytime we try to submit because if the the link is updated from
	// db, we should try with the new link. However, if we get once at the start of the
	// watcher and set it as a field, then the submission will NEVER get evaluated if the
	// submission link was wrong until the system is restarted
	_, spd, err := wt.probSerConfig.GetStandardProblemByID(ctx, cfSubStat.ProblemID)
	if err != nil {
		wt.logger.Errorf(
			"problem service config encountered error while getting standard probem with id %v",
			cfSubStat.ProblemID,
		)
		return false
	}
	if spd.SiteProblemCode == nil {
		wt.logger.Errorf(
			"standard problem with id %v has submission link as nil",
			cfSubStat.ProblemID,
		)
		return false
	}

	// construct the request
	req := cfSubRequest{
		submissionID:    wt.submissionID,
		solution:        solution,
		language:        language,
		siteProblemCode: *spd.SiteProblemCode,
	}

	// request master for submission
	wt.postman.postMail(
		mail{
			from:     wt.mailID,
			to:       mailNyxMaster,
			body:     req,
			priority: prNyxMstSubReq,
		},
	)

	wt.logger.Debug("requested master for submission")

	// submission success. do not update state to failure
	subSuccess = true

	return false
}

func (wt *nyxWatcher) handleSubResult(resMail mail) bool {
	res := resMail.body.(cfSubResult)

	// use a context to avoid indefinite wait
	ctx, cancel := wt.getInternalQueryCtx(time.Second * 5)
	defer cancel()

	// assume something would go wrong and update state to failure
	var updateToFailure bool = true
	var updateToFailurePtr *bool = &updateToFailure

	defer func() {
		if !*updateToFailurePtr {
			return
		}
		wt.updateSubStateToFailure(ctx)
	}()

	if res.err != nil {
		wt.logger.Errorf("master encountered error during submission")
		// state will be updated to failure by derfer

		return false
	}

	// start a transaction to update submission tables
	tx, err := service.GetNewTransaction(ctx)
	if err != nil {
		wt.logger.Error(
			"cannot get a transaction to update submission tables after success: %v",
			err,
		)
		// state will be updated to failure by derfer

		return false
	}
	defer tx.Rollback(ctx)

	// get a new query tool with the transaction
	qtx := wt.DB.WithTx(tx)

	// insert the result into db
	if _, err := qtx.InsertCfSubmission(
		ctx, database.InsertCfSubmissionParams{
			CfSubID:             res.status.CfSubID,
			SubmissionID:        wt.submissionID,
			TimeConsumedMillis:  res.status.TimeConsumedMillis,
			MemoryConsumedBytes: res.status.MemoryConsumedBytes,
			PassedTestCount:     res.status.PassedTestCount,
		},
	); err != nil {
		// if its not a unique key error, the watch has not yet ended
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != flux_errors.CodeUniqueConstraint {
			wt.logger.Errorf(
				"error occurred while inserting cf submission (%v) into db: %v",
				res.status, err,
			)
			return false
		}

		// unique error suggest that some watcher has already completed the submission and inserted into db
		wt.logger.Warnf(
			"unique error occurred while inserting cf submission (%v) into db: %v. ending watch assuming its a duplicate entry",
			res.status, err,
		)
	}

	// update submission state
	if err = wt.updateCfSubmissionState(ctx, qtx, res.status.Verdict); err != nil {
		wt.logger.Errorf(
			"failed to update submission state to %v after success",
			res.status.Verdict,
		)
		return false
	}

	// commit transaction
	if err = tx.Commit(ctx); err != nil {
		wt.logger.Errorf(
			"failed to commit transaction after success and updating submission tables: %v",
			err,
		)
		return false
	}

	// submission success. no need to update to failure
	updateToFailure = false

	wt.postman.postMail(mail{
		from:     wt.mailID,
		to:       mailNyxManager,
		body:     res,
		priority: prNyxMgrSubResult,
	})

	wt.logger.Debugf(
		"request was submitted to %v successfully. informed manager. ending watch",
		wt.platform,
	)

	return true
}

func (wt *nyxWatcher) getInternalQueryCtx(timeout time.Duration) (context.Context, context.CancelFunc) {
	return getContextWithKeys(timeout, internalSubmissionQuery, problem_service.InternalProblemQuery)
}

func (wt *nyxWatcher) updateSubStateToFailure(ctx context.Context) {
	if err := wt.updateCfSubmissionState(ctx, wt.DB, SubStatusFluxFailed); err != nil {
		wt.logger.Errorf(
			"submission failed but failed to update state to %v in db",
			SubStatusFluxFailed,
		)
	}
}

// the status of submission should always be in the unidirectional passage of stages:
// (flux_queued, flux_failed) -> (not_sink_states) -> (sink_states)
// however submission can posses any state interchangebly in a particular stage
func (wt *nyxWatcher) updateCfSubmissionState(ctx context.Context, qtx *database.Queries, state string) error {
	// get the status from db
	dbSub, err := wt.subQrr.getSubmission(ctx, wt.submissionID)
	if err != nil {
		wt.logger.Errorf(
			"failed to get submission from db while updating its state to %v",
			state,
		)
		return err
	}

	// cast it to dbCfSub
	dbCfSub, ok := dbSub.(dbCfSubStatus)
	if !ok {
		err := fmt.Errorf(
			"%w, cannot cast submission status queried from subStatMgr to dbCfSubStatus",
			flux_errors.ErrInternal,
		)
		wt.logger.Error(err)
		return err
	}

	// check if the state can be changed

	// if state is already a sink state then state cannot be updated any more
	if isCfSubSinkState(dbCfSub.State) {
		err := fmt.Errorf(
			"%w, cannot change state of the submission to (%v) once it gets to %v sink state",
			flux_errors.ErrInvalidRequest,
			dbCfSub.State,
			platformCodeforces,
		)
		wt.logger.Error(err)
		return err
	}

	// if state is non-sink-cf-state then, it cannot be changed to flux-states anymore
	if (dbCfSub.State != SubStatusFluxQueued && dbCfSub.State != SubStatusFluxFailed) &&
		(state == SubStatusFluxFailed || state == SubStatusFluxQueued) {
		err := fmt.Errorf(
			"%w, cannot change state of submission from non-sink-cf-state (%v) to flux-state (%v)",
			flux_errors.ErrInvalidRequest,
			dbCfSub.State,
			state,
		)
		wt.logger.Error(err)
		return err
	}

	// update
	_, err = wt.subQrr.updateSubmission(
		ctx, qtx, wt.submissionID, state,
	)

	return err
}

func (wt *nyxWatcher) informManagerAboutWatchEnd(res cfSubResult) {
	wt.postman.postMail(
		mail{
			from:     wt.mailID,
			to:       mailNyxManager,
			body:     res,
			priority: prNyxMgrWtEnded,
		},
	)
	wt.logger.Debug("informed manager that my watch has ended")
}

func (wt *nyxWatcher) recieveMail(mail mail) {
	wt.mailBox.Add(mail)
}

func (wt *nyxWatcher) getMailID() mailID {
	return wt.mailID
}
