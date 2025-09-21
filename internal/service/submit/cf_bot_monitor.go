package submission_service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (monitor *cfBotMonitor) start(cfBaseUrl string) error {
	// get custom logger
	logger := monitor.getLogger()

	// parse the base url
	parsedUrl, err := url.Parse(cfBaseUrl)
	if err != nil {
		err := fmt.Errorf(
			"%w, cannot parse url %s, %w",
			flux_errors.ErrMonitorStart,
			cfBaseUrl,
			err,
		)
		logger.Error(err)
		return err
	}
	monitor.cfBaseUrl = parsedUrl

	// check if postman is non nil
	if monitor.postman == nil {
		err = fmt.Errorf(
			"%w, non-nil postman is expected",
			flux_errors.ErrMonitorStart,
		)
		logger.Error(err)
		return err
	}

	// initialize status
	monitor.status = make(map[int64]cfSubStatus)

	// make the new sub channel
	monitor.newSubs = make(chan cfSubStatus, 10)

	go monitor.monitor()
	
	logger.Debugf("monitor %v started successfully", monitor.botName)
	
	return nil
}

func (monitor *cfBotMonitor) monitor() {
	logger := monitor.getLogger()

	// loop
	for {
		// random sleep to avoid infinite cycles
		time.Sleep(time.Second * 5)

		// empty the new sub alerts
		monitor.emptyNewSubAlerts()

		// check if there are any running submissions
		perform := monitor.shouldPerformCycle()
		if !perform {
			continue
		}

		// get latest submissions
		subStatus, err := monitor.getLatestSubmissions(1, 100)
		if err != nil {
			logger.Errorf("monitor unable to query for submissions in its loop cycle")
			continue
		}

		// update the submissions
		subStatusMap := make(map[int64]cfSubStatus, len(subStatus))
		for _, status := range subStatus {
			subStatus[status.id] = status
		}

		// update the record
		monitor.Lock()
		monitor.status = subStatusMap
		monitor.Unlock()
	}
}

func (monitor *cfBotMonitor) emptyNewSubAlerts() {
	monitor.Lock()
	defer monitor.Unlock()

	for {
		select {
		case newSub := <-monitor.newSubs:
			monitor.status[newSub.id] = newSub
		default:
			return
		}
	}
}

func (monitor *cfBotMonitor) newSubAlert(sub cfSubStatus) {
	monitor.newSubs <- sub
}

func (monitor *cfBotMonitor) getLogger() *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			"monitor_name": monitor.botName,
		},
	)
}

func (monitor *cfBotMonitor) getLatestSubmissions(
	subOffset int32, // 1 based offset (eg., if you want the latest submission, set this as 1)
	subCount int32,
) ([]cfSubStatus, error) {
	// get logger
	logger := monitor.getLogger()

	urlParams := url.Values{}
	urlParams.Add("handle", monitor.botName)
	urlParams.Add("from", string(subOffset))
	urlParams.Add("count", string(subCount))

	baseUrlCopy := *monitor.cfBaseUrl
	baseUrlCopy.RawQuery = urlParams.Encode()

	// execute
	res, err := http.Get(monitor.cfBaseUrl.String())
	if err != nil {
		err = fmt.Errorf(
			"%w, cannot make request to %v's api for monitoring, %w",
			flux_errors.ErrInternal,
			platformCodeforces,
			err,
		)
		logger.Error(err)
		return nil, err
	}

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
		logger.Error(err)
		return nil, err
	}

	// check if verdict is OK
	if resJson.Status == "FAILED" {
		err = fmt.Errorf(
			"%w, cf monitor response return FAILED status, %s",
			flux_errors.ErrHttpResponse,
			resJson.Comment,
		)
		logger.Error(err)
		return nil, err
	} else if resJson.Status != "OK" {
		err = fmt.Errorf(
			"%w, response status is not \"OK\"",
			flux_errors.ErrHttpResponse,
		)
		logger.WithField("response", resJson).Error(err)
		return nil, err
	}

	return resJson.Result, nil
}

func (monitor *cfBotMonitor) shouldPerformCycle() bool {
	monitor.Lock()
	defer monitor.Unlock()

	for _, status := range monitor.status {
		if !isCfSubSinkState(status.verdict) {
			return true
		}
	}

	return false
}
