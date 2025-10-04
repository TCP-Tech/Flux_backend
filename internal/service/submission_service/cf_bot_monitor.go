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
	if monitor.botName == "" {
		panic("monitor's bot name is empty")
	}

	monitor.logger = logrus.WithFields(
		logrus.Fields{
			"from": monitor.botName + "-bot_montior",
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

	monitor.subStatMap = make(map[int64]cfSubStatus)
	monitor.newSubs = make(chan cfSubStatus, 10)

	go monitor.monitor()

	monitor.logger.Infof("monitor %v started successfully", monitor.botName)
}

func (monitor *cfBotMonitor) monitor() {
	// loop
	for {
		// random sleep to avoid infinite cycles
		time.Sleep(time.Second * 5)

		// empty the new sub alerts
		monitor.emptyNewSubAlerts()

		// check if there are any running submissions on cf
		shouldQuery := monitor.shouldQueryCf()

		// found active submissions

		var subStatus []cfSubStatus = make([]cfSubStatus, 0)

		if shouldQuery {
			monitor.logger.Debugf(
				"monitor found active submissions. querying %v for their status",
				platformCodeforces,
			)

			// get latest submissions
			subStatus, err := monitor.querySubmissions(1, 100)
			if err != nil {
				monitor.logger.Errorf("monitor unable to query for submissions in its loop cycle")
				continue
			}

			// update the submissions
			subStatusMap := make(map[int64]cfSubStatus, len(subStatus))
			for _, status := range subStatus {
				subStatusMap[status.CfSubID] = status
			}

			// update the record
			monitor.Lock() // lock to prevent race modification of state
			monitor.subStatMap = subStatusMap
			monitor.Unlock() // state of monitor is not gonna change any more. so unlock.
		} else {
			monitor.Lock()
			for _, stat := range monitor.subStatMap {
				subStatus = append(subStatus, stat)
			}
			monitor.Unlock()
		}

		// update results into db
		monitor.updateEntriesIntoDB(subStatus)
	}
}

// TimeComplexity: O(nlogn) for sorting submissions and
// 2 db calls for updating submissions table and cf_submissions table
func (monitor *cfBotMonitor) updateEntriesIntoDB(httpEntries []cfSubStatus) {
	// create a context for the whole update
	updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// cancel always at the end to release resources. its reentrant meaning, it can be safely called multiple times
	defer cancel()

	// sort the slice with the cf_sub_id id
	statCmpr := func(i, j int) bool { return httpEntries[i].CfSubID < httpEntries[j].CfSubID }
	sort.Slice(httpEntries, statCmpr)

	// collect all the ids into a slice
	ids := make([]int64, 0)
	for _, stat := range httpEntries {
		ids = append(ids, stat.CfSubID)
	}

	// get the dbEntries from db
	dbEntries, err := monitor.getBulkEntries(updateCtx, ids)
	if err != nil {
		monitor.logger.Error("cannot update cf submissions into db. failed to get bulk entries")
		return
	}

	// sort the entries as well
	sort.Slice(dbEntries, statCmpr)

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
		monitor.logger.Debug("no submissions to update entries into db in current cycle")
		return
	}

	// check which submissions are not latest and update them selectively
	// start a transaction to update submissions table and cf_submissions table as an atomic operation\
	tx, err := service.GetNewTransaction(updateCtx)
	if err != nil {
		monitor.logger.Error("failed to get new transaction to update cf submissions into db. aborting update")
		return
	}
	defer tx.Rollback(updateCtx)

	// create a queries object with this transaction
	qtx := monitor.DB.WithTx(tx)

	// update submissions
	if err = monitor.subStatMgr.bulkUpdateSubmissionState(updateCtx, qtx, subIDs, states); err != nil {
		monitor.logger.Error("encountered error from db while updating submissions table in current cycle")
		return
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
		return
	}

	// commit the transaction
	if err := tx.Commit(updateCtx); err != nil {
		monitor.logger.Error("failed to commit transaction after updating submissions in db")
		return
	}
}

func (monitor *cfBotMonitor) getBulkEntries(ctx context.Context, ids []int64) ([]cfSubResult, error) {
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

func (monitor *cfBotMonitor) emptyNewSubAlerts() {
	monitor.Lock()
	defer monitor.Unlock()

	for {
		select {
		case newSub := <-monitor.newSubs:
			monitor.logger.Debugf("found new submission alert with id %v", newSub.CfSubID)
			monitor.subStatMap[newSub.CfSubID] = newSub
		default:
			return
		}
	}
}

func (monitor *cfBotMonitor) newSubAlert(sub cfSubStatus) {
	monitor.newSubs <- sub
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	return resJson.Result, nil
}

func (monitor *cfBotMonitor) getLatestSubmission(prevSub int64) (cfSubStatus, error) {
	// get the max submission id currently
	var maxSubID int64 = 0
	var status cfSubStatus

	monitor.Lock()
	for k, v := range monitor.subStatMap {
		if k > maxSubID {
			maxSubID = k
			status = v
		}
	}
	monitor.Unlock()

	if maxSubID > 0 && prevSub < maxSubID {
		return status, nil
	}

	monitor.logger.Debugf(
		"previous submission id %v is same as max submission id, querying cf",
		prevSub,
	)

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
	monitor.logger.Debugf("alerted monitor %v about new submission %v", monitor.botName, latestSub.CfSubID)
	go monitor.newSubAlert(latestSub)

	return latestSub, nil
}

func (monitor *cfBotMonitor) shouldQueryCf() bool {
	monitor.Lock()
	defer monitor.Unlock()

	for _, status := range monitor.subStatMap {
		if !isCfSubSinkState(status.Verdict) {
			return true
		}
	}

	return false
}
