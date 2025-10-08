package submission_service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (mgr *nyxBotMgr) start() error {
	mgr.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailNyxBotMgr,
		},
	)

	if mgr.postman == nil {
		panic("nyx bot manager expects non-nil postman")
	}
	if mgr.cfQueryUrl == "" {
		panic("bot manager expects non-empty cf query url")
	}
	if mgr.subStatMgr == nil {
		panic("bot manager expects non-nil submission status manager")
	}

	// initialize fields
	mgr.monitors = make(map[string]*cfBotMonitor)
	mgr.mailBox = make(chan mail, 10)
	mgr.distribution = make(map[uuid.UUID]nyxSlaveDist)

	go mgr.processMails()
	mgr.logger.Infof("bot manager started processing mails")

	return nil
}

func (mgr *nyxBotMgr) processMails() {
	for ml := range mgr.mailBox {
		switch body := ml.body.(type) {
		case mgrRefreshBots:
			mgr.refreshBots(body.bots, body.slaves)
		case mnrStopped:
			mgr.Lock()
			delete(mgr.monitors, string(body))
			mgr.Unlock()

			mgr.logger.Warnf("bot %v stopped. removed it from inventory", body)
		default:
			mgr.logger.Errorf("recieved unknown mail %v", ml)
		}
	}
}

func (mgr *nyxBotMgr) refreshBots(bots []Bot, slaves []slaveInfo) {
	mgr.Lock()
	defer mgr.Unlock()

	allBots := make(map[string]Bot)
	for _, bot := range bots {
		allBots[bot.Name] = bot
	}

	// send life signals to old bots
	for _, oldBotMonitor := range mgr.monitors {
		if _, ok := allBots[oldBotMonitor.botName]; !ok {
			// send stop signal to it
			mgr.postman.postMail(mail{
				from: mailNyxBotMgr,
				to:   oldBotMonitor.mailID,
				body: stop(time.Now()),
			})
			mgr.logger.Debugf("send stop signal to bot monitor %v", oldBotMonitor.mailID)
		} else {
			// send keep alive signal to it
			mgr.postman.postMail(mail{
				from: mailNyxBotMgr,
				to:   oldBotMonitor.mailID,
				body: keepAlive(time.Now()),
			})
			mgr.logger.Debugf("send keep alive signal to bot monitor %v", oldBotMonitor.mailID)
		}
	}

	// create monitors for new bots
	for _, newBot := range bots {
		if _, ok := mgr.monitors[newBot.Name]; ok {
			continue
		}

		// create a new monitor for that bot
		monitor := cfBotMonitor{
			botName:    newBot.Name,
			mailID:     mailID(fmt.Sprintf("mail@bot-%s", newBot.Name)),
			postman:    mgr.postman,
			DB:         mgr.DB,
			subStatMgr: mgr.subStatMgr,
		}

		monitor.start(mgr.cfQueryUrl)
		mgr.monitors[newBot.Name] = &monitor

		// register with postman
		mgr.postman.RegisterMailClient(monitor.mailID, &monitor)

		mgr.logger.Infof("created a new monitor for bot %v and registered with postman", monitor.mailID)
	}

	// distribute bots to slaves fairly

	if len(slaves) == 0 {
		mgr.logger.WithField("ids", slaves).Warn("length of slaves is 0")
		mgr.distribution = make(map[uuid.UUID]nyxSlaveDist)
		return
	}

	if len(bots) == 0 {
		mgr.logger.WithField("bots", bots).Warn("length of bots is 0")
		mgr.distribution = make(map[uuid.UUID]nyxSlaveDist)
		for _, info := range slaves {
			mgr.distribution[info.slaveID] = nyxSlaveDist{slaveInfo: info}
		}
		return
	}

	if len(bots) < len(slaves) {
		mgr.logger.Warnf("number of bots (%v) is less than number of slaves (%v)", len(bots), len(slaves))
	}

	// calculate ceil number of bots per slave
	ceilBots := float64((len(bots) + len(slaves) - 1))
	floatSlaveCount := float64(len(slaves))
	botsPerSlave := int(ceilBots / floatSlaveCount)
	mgr.logger.Infof("bots per slave in current dist: %v", botsPerSlave)

	// get all slaves and their bots into a map
	curDist := make(map[uuid.UUID]nyxSlaveDist)
	for _, sinfo := range slaves {
		// put the slave into curDist
		svent, ok := mgr.distribution[sinfo.slaveID]
		curDist[sinfo.slaveID] = nyxSlaveDist{
			slaveInfo:   sinfo,
			bots:        make([]Bot, 0),
			lastUsedBot: -1,
		}

		// check if the slave was present before
		if !ok {
			mgr.logger.Debugf("new slave found in current dist: %v", sinfo.slvMailId)
			continue
		}

		// check for array index out of bounds
		if svent.lastUsedBot < 0 || svent.lastUsedBot >= len(svent.bots) {
			continue
		}

		// check if their last used bot is present in the current dist
		prevBot := svent.bots[svent.lastUsedBot]
		_, ok = allBots[prevBot.Name]
		if !ok {
			mgr.logger.Warnf(
				"bot %v was used by %v. but absent in the current distribution",
				prevBot.Name, svent.slvMailId,
			)
			continue
		}

		// assign their last used bot to them
		svent.bots = []Bot{prevBot}
		svent.lastUsedBot = 0
		curDist[svent.slaveID] = svent

		// delete from all bots
		delete(allBots, prevBot.Name)
	}

	// get remaining bots into a slice
	remBots := make([]Bot, 0, len(allBots))
	for _, bot := range allBots {
		remBots = append(remBots, bot)
	}

	i := 0 // pointer over remaining bots

	// distribute the remaining bots
	for id, svent := range curDist {
		// safety check
		if svent.bots == nil {
			svent.bots = make([]Bot, 0)
		}
		// assign bots to it till it has botsPerSlave bots or num bots exhausted
		for len(svent.bots) < botsPerSlave && i < len(remBots) {
			svent.bots = append(svent.bots, remBots[i])
			i++
		}
		curDist[id] = svent
	}

	// assign to current distribution
	mgr.distribution = curDist

	mgr.logger.Debugf("completed distribution")
}

func (mgr *nyxBotMgr) getBot(slaveID uuid.UUID) (Bot, error) {
	mgr.Lock()
	defer mgr.Unlock()

	sdist, ok := mgr.distribution[slaveID]
	if !ok {
		err := fmt.Errorf(
			"%w, slave with id %v was not found in the inventory but asked for a bot",
			flux_errors.ErrNotFound,
			getShortUUID(slaveID, 5),
		)
		mgr.logger.Error(err)
		return Bot{}, err
	}

	// check if the slave has any bots
	if len(sdist.bots) == 0 {
		err := fmt.Errorf(
			"%w, slave %v requested for a bot but has no bots assigned",
			errNoBots,
			sdist.slvMailId,
		)
		mgr.logger.Error(err)
		return Bot{}, err
	}

	sdist.lastUsedBot = (sdist.lastUsedBot + 1) % len(sdist.bots)
	nextBot := sdist.bots[sdist.lastUsedBot]

	mgr.distribution[slaveID] = sdist

	return nextBot, nil
}

func (mgr *nyxBotMgr) getLatestBotSubmission(botName string) (cfSubStatus, error) {
	// get monitor
	mgr.Lock()
	monitor, ok := mgr.monitors[botName]
	mgr.Unlock()

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
	mgr.logger.Debugf("asking monitor %v for latest submission", monitor.mailID)
	stat, err := monitor.getLatestSubmission()

	return stat, err
}

func (mgr *nyxBotMgr) recieveMail(ml mail) {
	mgr.mailBox <- ml
}

func (mgr *nyxBotMgr) getMailID() mailID {
	return mailNyxBotMgr
}
