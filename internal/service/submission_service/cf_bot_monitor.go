package submission_service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service"
)

func (monitor *cfBotMonitor) start(cfQueryUrl string) {
	if monitor.mailID == "" {
		panic("monitor's mailID is empty")
	}
	if monitor.botName == "" {
		panic("monitor's bot name is empty")
	}

	monitor.logger = logrus.WithFields(
		logrus.Fields{
			"from": monitor.mailID,
		},
	)

	// parse the base url
	parsedUrl, err := url.Parse(cfQueryUrl)
	if err != nil {
		panic("cannot parse cfQueryUrl: " + cfQueryUrl)
	}
	monitor.cfQueryUrl = parsedUrl

	// check if postman is non nil
	if monitor.postman == nil {
		panic("monitor expects non-nil postman")
	}

	if monitor.DB == nil {
		panic("monitor expects non-nil db object")
	}

	if monitor.subStatMgr == nil {
		panic("monitor expects non-nil submission status manager")
	}

	if monitor.subStatMap == nil {
		monitor.subStatMap = make(map[int64]cfSubStatus)
	}
	if monitor.mailBox == nil {
		monitor.mailBox = make(chan mail, 10)
	}

	monitor.stopDecision = mnrStopDecision{endLife: false, ltsSignal: time.Now()}

	go monitor.processMails()

	monitor.logger.Infof("monitor %v started successfully", monitor.mailID)
}

func (monitor *cfBotMonitor) processMails() {
	defer func() {
		monitor.logger.Info("exiting process mails")
	}()

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case ml := <-monitor.mailBox:
			switch body := ml.body.(type) {
			case stop:
				stopTime := time.Time(body)
				if stopTime.After(monitor.stopDecision.ltsSignal) {
					monitor.stopDecision.ltsSignal = stopTime
					monitor.stopDecision.endLife = true
					monitor.stopDecision.ltsStopDecision = stopTime
				}
			case keepAlive:
				kaTime := time.Time(body)
				if kaTime.After(monitor.stopDecision.ltsSignal) {
					monitor.stopDecision.endLife = false
					monitor.stopDecision.ltsSignal = kaTime
				}
			case mnrSubAlert:
				status := cfSubStatus(body)
				monitor.logger.Debugf("found new submission alert with id %v", status)

				// put the entry into map
				monitor.subStatMap[status.CfSubID] = status

				// update stop descision regardless of endLife
				monitor.stopDecision.ltsStopDecision = time.Now()
			case mnrUpdateStopDecision:
				tm := time.Time(body)
				if monitor.stopDecision.endLife {
					monitor.stopDecision.ltsStopDecision = tm
				}
			default:
				monitor.logger.Errorf("recieved unknown mail %v", ml)
			}
		case <-ticker.C:
			if !monitor.stopDecision.endLife {
				monitor.monitor()
				continue
			}

			expiry := monitor.stopDecision.ltsStopDecision.Add(time.Minute * 5)
			if time.Now().Before(expiry) {
				monitor.monitor()
				continue
			}

			// inform manager that monitor is dead
			monitor.postman.postMail(mail{
				from: monitor.mailID,
				to:   mailNyxBotMgr,
				body: mnrStopped(monitor.mailID),
			})
			monitor.logger.Warnf("monitor stopped")
			return
		}
	}
}

func (monitor *cfBotMonitor) monitor() {
	// check if there are any running submissions on cf
	shouldQuery := monitor.shouldQueryCf()
	isColdStart := len(monitor.subStatMap) == 0

	var subStatus []cfSubStatus = make([]cfSubStatus, 0)

	if shouldQuery || isColdStart {
		monitor.logger.Debugf(
			"querying %v for submissions status",
			platformCodeforces,
		)

		// get latest submissions by querying cf
		var err error
		subStatus, err = monitor.querySubmissions(1, 50)
		if err != nil {
			monitor.logger.Errorf("monitor unable to query for submissions in its loop cycle")
			return
		}

		// update the submissions
		subStatusMap := make(map[int64]cfSubStatus, len(subStatus))
		for _, status := range subStatus {
			subStatusMap[status.CfSubID] = status
		}

		// update the record
		monitor.subStatMap = subStatusMap
	} else {
		for _, stat := range monitor.subStatMap {
			subStatus = append(subStatus, stat)
		}
	}

	// update results into db
	updateStopDecision := monitor.updateEntriesIntoDB(subStatus)

	if updateStopDecision {
		// inform itself to update
		monitor.postman.postMail(mail{
			from: monitor.mailID,
			to:   monitor.mailID,
			body: mnrUpdateStopDecision(time.Now()),
		})

		monitor.logger.Debugf("asked to update my stop decision")
	}
}

// TimeComplexity: O(nlogn) for sorting submissions and
// 2 db calls for updating submissions table and cf_submissions table
func (monitor *cfBotMonitor) updateEntriesIntoDB(httpEntries []cfSubStatus) bool {
	// create a context for the whole update
	updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// cancel always at the end to release resources. its reentrant meaning, it can be safely called multiple times
	defer cancel()

	// sort the slice with the cf_sub_id id
	sort.Slice(httpEntries, func(i, j int) bool { return httpEntries[i].CfSubID < httpEntries[j].CfSubID })

	// collect all the ids into a slice
	ids := make([]int64, 0)
	for _, stat := range httpEntries {
		ids = append(ids, stat.CfSubID)
	}

	// get the dbEntries from db
	dbEntries, err := monitor.getBulkEntries(updateCtx)
	if err != nil {
		monitor.logger.Error("cannot update cf submissions into db. failed to get bulk entries")
		return true
	}

	// sort the entries as well
	sort.Slice(dbEntries, func(i, j int) bool { return dbEntries[i].status.CfSubID < dbEntries[j].status.CfSubID })

	cfIDs := make([]int64, 0, len(httpEntries))
	subIDs := make([]uuid.UUID, 0, len(httpEntries))
	states := make([]string, 0, len(httpEntries))
	times := make([]int32, 0, len(httpEntries))
	memories := make([]int32, 0, len(httpEntries))
	passedTestCounts := make([]int32, 0, len(httpEntries))

	// use a 2 pointer technique to decide which submissions to update
	for i, j := 0, 0; i < len(httpEntries) && j < len(dbEntries); {
		httpEntry := httpEntries[i]
		dbEntry := dbEntries[j]
		if httpEntry.CfSubID > dbEntry.status.CfSubID {
			j++
			continue
		}
		if httpEntry.CfSubID < dbEntry.status.CfSubID {
			i++
			continue
		}

		// check if it has been changed
		if !httpEntry.equal(dbEntry.status) {
			// update
			cfIDs = append(cfIDs, httpEntry.CfSubID)
			subIDs = append(subIDs, dbEntry.submissionID)
			states = append(states, httpEntry.Verdict)
			times = append(times, httpEntry.TimeConsumedMillis)
			memories = append(memories, httpEntry.MemoryConsumedBytes)
			passedTestCounts = append(passedTestCounts, httpEntry.PassedTestCount)
		}

		i++
		j++
	}

	// check for trivial updates
	if len(cfIDs) == 0 {
		return false
	}

	// check which submissions are not latest and update them selectively
	// start a transaction to update submissions table and cf_submissions table as an atomic operation\
	tx, err := service.GetNewTransaction(updateCtx)
	if err != nil {
		monitor.logger.Error("failed to get new transaction to update cf submissions into db. aborting update")
		return true
	}
	defer tx.Rollback(updateCtx)

	// create a queries object with this transaction
	qtx := monitor.DB.WithTx(tx)

	// update submissions
	if err = monitor.subStatMgr.bulkUpdateSubmissionState(updateCtx, qtx, subIDs, states); err != nil {
		monitor.logger.Error("encountered error from db while updating submissions table in current cycle")
		return true
	}

	// update cf_submissions
	if err = qtx.BulkUpdateCfSubmission(updateCtx, database.BulkUpdateCfSubmissionParams{
		Ids:              cfIDs,
		Times:            times,
		Memories:         memories,
		PassedTestCounts: passedTestCounts,
	}); err != nil {
		flux_errors.HandleDBErrors(
			err, errMsgs,
			"failed to update cf_submissions table while updating submission entries into db",
		)
		monitor.logger.Error("encountered error from db while updating cf_submissions table in current cycle")
		return true
	}

	// commit the transaction
	if err := tx.Commit(updateCtx); err != nil {
		monitor.logger.Error("failed to commit transaction after updating submissions in db")
		return true
	}

	return true
}

func (monitor *cfBotMonitor) getBulkEntries(ctx context.Context) ([]cfSubResult, error) {
	entries, err := monitor.DB.GetBulkCfSubmission(ctx, cfSinkStates)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err, errMsgs, "failed to get bulk cf submission entries from db",
		)
		return nil, err
	}

	res := make([]cfSubResult, 0)
	for _, entry := range entries {
		res = append(res, cfSubResult{
			submissionID: entry.SubmissionID,
			status: cfSubStatus{
				CfSubID:             entry.CfSubID,
				Verdict:             entry.State,
				TimeConsumedMillis:  entry.TimeConsumedMillis,
				MemoryConsumedBytes: entry.MemoryConsumedBytes,
				PassedTestCount:     entry.PassedTestCount,
			},
		})
	}

	return res, nil
}

func (monitor *cfBotMonitor) querySubmissions(
	subOffset int32, // 1 based offset (eg., if you want the latest submission, set this as 1)
	subCount int32,
) ([]cfSubStatus, error) {
	urlParams := url.Values{}
	urlParams.Add("handle", monitor.botName)
	urlParams.Add("from", strconv.Itoa(int(subOffset)))
	urlParams.Add("count", strconv.Itoa(int(subCount)))

	baseUrlCopy := *monitor.cfQueryUrl
	baseUrlCopy.RawQuery = urlParams.Encode()

	// create a context to avoid indefinite wait
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get a new request with the context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseUrlCopy.String(), nil)
	if err != nil {
		err := fmt.Errorf("%w, failed to create http request with ctx: %w", flux_errors.ErrInternal, err)
		monitor.logger.Error(err)
		return nil, err
	}

	// Use a default HTTP client
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		// Error here could be a timeout from the context or a network issue
		err = fmt.Errorf(
			"%w, failed to get response from %v: %w",
			flux_errors.ErrHttpResponse, baseUrlCopy.String(), err,
		)
		monitor.logger.Error(err)
		return nil, err
	}
	defer res.Body.Close()
	monitor.logger.Debugf("recieved response from %v", baseUrlCopy)

	// response to be parsed
	var resJson struct {
		Status  string        `json:"status"`
		Result  []cfSubStatus `json:"result"`
		Comment string        `json:"comment"`
	}

	// decode
	if err = json.NewDecoder(res.Body).Decode(&resJson); err != nil {
		err = fmt.Errorf(
			"%w, cannot decode %v to %T, %w",
			flux_errors.ErrHttpResponse,
			res.Body,
			resJson,
			err,
		)
		monitor.logger.Error(err)
		return nil, err
	}

	// check if verdict is OK
	if resJson.Status == "FAILED" {
		err = fmt.Errorf(
			"%w, cf monitor response return FAILED status, %s",
			flux_errors.ErrHttpResponse,
			resJson.Comment,
		)
		monitor.logger.Error(err)
		return nil, err
	} else if resJson.Status != "OK" {
		err = fmt.Errorf(
			"%w, response status is not \"OK\"",
			flux_errors.ErrHttpResponse,
		)
		monitor.logger.WithField("response", resJson).Error(err)
		return nil, err
	}

	// sometimes cf can report empty verdict. Although its considered as non-sink-cf-state,
	// replace it with TESTING for clarity
	for _, stat := range resJson.Result {
		if stat.Verdict == "" {
			stat.Verdict = "TESTING"
		}
	}

	return resJson.Result, nil
}

func (monitor *cfBotMonitor) getLatestSubmission() (cfSubStatus, error) {
	// query for latest submission
	latestSubs, err := monitor.querySubmissions(1, 1)
	if err != nil {
		return cfSubStatus{}, err
	}
	// check number of submissions in the result
	if len(latestSubs) != 1 {
		err := fmt.Errorf(
			"%w, quried for 1 latest submission but got: %v",
			flux_errors.ErrHttpResponse,
			latestSubs,
		)
		monitor.logger.Error(err)
		return cfSubStatus{}, err
	}

	latestSub := latestSubs[0]

	// alert about new submission because if there is no submission for a long time
	// then shouldPerformCycle of botMonitor will return false always once everything settles.
	// There is no way for monitor to know about submission without querying. So, this helps in lazy
	// querying to avoid unnecessary network calls.
	monitor.postman.postMail(mail{
		from: monitor.mailID,
		to:   monitor.mailID,
		body: mnrSubAlert(latestSub),
	})
	monitor.logger.Debugf("alerted myself about new submission %v", latestSub.CfSubID)

	return latestSub, nil
}

func (monitor *cfBotMonitor) shouldQueryCf() bool {
	for _, status := range monitor.subStatMap {
		if !isCfSubSinkState(status.Verdict) {
			return true
		}
	}

	return false
}

func (monitor *cfBotMonitor) recieveMail(ml mail) {
	monitor.mailBox <- ml
}

func (monitor *cfBotMonitor) getMailID() mailID {
	return monitor.mailID
}
