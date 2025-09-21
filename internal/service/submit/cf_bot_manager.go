package submission_service

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (mgr *cfBotManager) start(botNames []string) error {
	// get custom logger
	logger := mgr.getLogger()

	if mgr.postman == nil {
		err := fmt.Errorf(
			"%w, non-nil postman is expected",
			flux_errors.ErrInternal,
		)
		logger.Error(err)
		return err
	}

	// initialize fields
	mgr.monitors = make(map[string]*cfBotMonitor, len(botNames))

	// start bots
	for _, botName := range botNames {
		monitor := cfBotMonitor{
			botName: botName,
			postman: mgr.postman,
		}

		// start
		logger.Debugf("starting monitor %s", botName)
		err := monitor.start(mgr.cfBaseUrl)
		if err != nil {
			logger.Errorf("failed to start monitor %v", botName)
			return err
		}

		// register
		mgr.monitors[botName] = &monitor
	}

	return nil
}

func (mgr *cfBotManager) getLogger() *logrus.Entry {
	return logrus.WithFields(
		logrus.Fields{
			"mailID": mailCfBotMgr,
		},
	)
}


// not thread safe. make sure a bot's status is queried by only one goroutine at a time
func (mgr *cfBotManager) getLatestBotSubmission(botName string) (cfSubStatus, error) {
	logger := mgr.getLogger()

	// get monitor
	monitor, ok := mgr.monitors[botName]
	if !ok {
		// make this internal error as well so it wont reach end user by mistake
		err := fmt.Errorf(
			"%w, %w, no monitor found for bot %v",
			flux_errors.ErrInternal,
			flux_errors.ErrNotFound,
			botName,
		)
		logger.Error(err)
		return cfSubStatus{}, err
	}

	// get latest submission
	stat, err := monitor.getLatestSubmissions(1, 1)
	if err != nil {
		logger.Error("monitor failed to get latest submission")
		return cfSubStatus{}, err
	}
	if len(stat) != 1 {
		err := fmt.Errorf(
			"%w, expected 1 latest submission from monitor %v but got %v",
			flux_errors.ErrInternal,
			botName,
			len(stat),
		)
		logger.WithField("response", stat).Error(err)
		return cfSubStatus{}, err
	}

	// alert about new submission
	go monitor.newSubAlert(stat[0])

	return stat[0], nil
}
