package submission_service

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (mgr *nyxBotMgr) start(bots []Bot) error {
	mgr.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailNyxBotMgr,
		},
	)

	if mgr.postman == nil {
		panic("nyx bot manager expects non-nil postman")
	}

	// initialize fields
	mgr.monitors = make(map[string]*cfBotMonitor, len(bots))

	// start bots
	// NOTE: currently only codeforces is supported as a platform. so only cfBotMonitors exist
	for _, bot := range bots {
		monitor := cfBotMonitor{
			botName:    bot.Name,
			postman:    mgr.postman,
			DB:         mgr.DB,
			subStatMgr: mgr.subStatMgr,
		}

		// start
		mgr.logger.Debugf("starting monitor %s", bot.Name)
		monitor.start(mgr.cfQueryUrl)

		// register
		mgr.monitors[bot.Name] = &monitor
	}

	return nil
}

func (mgr *nyxBotMgr) getLatestBotSubmission(botName string, prevSubID int64) (cfSubStatus, error) {
	// get monitor
	monitor, ok := mgr.monitors[botName]
	if !ok {
		err := fmt.Errorf(
			"%w, no monitor found for bot %v",
			flux_errors.ErrInternal,
			flux_errors.ErrNotFound,
			botName,
		)
		mgr.logger.Error(err)
		return cfSubStatus{}, err
	}

	// get latest submission
	stat, err := monitor.getLatestSubmission(prevSubID)

	return stat, err
}
