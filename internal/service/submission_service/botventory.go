package submission_service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (bv *nyxBotventory) start() {
	bv.logger = logrus.WithFields(
		logrus.Fields{
			"from": "botventory",
		},
	)
	bv.logger.Info("initialized")
	bv.inventory = make(map[uuid.UUID]nyxSlaventory)
}

func (bv *nyxBotventory) redistributeBots(bots []Bot, slaves []slaveInfo) {
	bv.Lock()
	defer bv.Unlock()

	if len(slaves) == 0 {
		bv.logger.WithField("ids", slaves).Warn("length of slaves is 0")
		bv.inventory = make(map[uuid.UUID]nyxSlaventory)
		return
	}

	if len(bots) == 0 {
		bv.logger.WithField("bots", bots).Warn("length of bots is 0")
		bv.inventory = make(map[uuid.UUID]nyxSlaventory)
		for _, info := range slaves {
			bv.inventory[info.slaveID] = nyxSlaventory{slaveInfo: info}
		}
		return
	}

	if len(bots) < len(slaves) {
		bv.logger.Warnf("number of bots (%v) is less than number of slaves (%v)", len(bots), len(slaves))
	}

	// calculate ceil number of bots per slave
	ceilBots := float64((len(bots) + len(slaves) - 1))
	floatSlaveCount := float64(len(slaves))
	botsPerSlave := int(ceilBots / floatSlaveCount)
	bv.logger.Infof("bots per slave in current dist: %v", botsPerSlave)

	// get all bots into a map
	allBots := make(map[string]Bot)
	for _, bot := range bots {
		allBots[bot.Name] = bot
	}

	// get all slaves and their bots into a map
	currentory := make(map[uuid.UUID]nyxSlaventory)
	for _, sinfo := range slaves {
		// put the slave into currentory
		svent, ok := bv.inventory[sinfo.slaveID]
		currentory[sinfo.slaveID] = nyxSlaventory{
			slaveInfo:   sinfo,
			bots:        make([]Bot, 0),
			lastUsedBot: -1,
		}

		// check if the slave was present before
		if !ok {
			bv.logger.Debugf("new slave found in current dist: %v", sinfo.slvMailId)
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
			bv.logger.Warnf(
				"bot %v was used by %v. but absent in the current distribution",
				prevBot.Name, svent.slvMailId,
			)
			continue
		}

		// assign their last used bot to them
		svent.bots = []Bot{prevBot}
		svent.lastUsedBot = 0
		currentory[svent.slaveID] = svent

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
	for id, svent := range currentory {
		// safety check
		if svent.bots == nil {
			svent.bots = make([]Bot, 0)
		}
		// assign bots to it till it has botsPerSlave bots or num bots exhausted
		for len(svent.bots) < botsPerSlave && i < len(remBots) {
			svent.bots = append(svent.bots, remBots[i])
			i++
		}
		currentory[id] = svent
	}

	// assign to current inventory
	bv.inventory = currentory

	bv.logger.Debugf("completed distribution")
}

func (bv *nyxBotventory) getBot(slaveID uuid.UUID) (Bot, error) {
	bv.Lock()
	defer bv.Unlock()

	svent, ok := bv.inventory[slaveID]
	if !ok {
		err := fmt.Errorf(
			"%w, slave with id %v was not found in the inventory but asked for a bot",
			flux_errors.ErrNotFound,
			getShortUUID(slaveID, 5),
		)
		bv.logger.Error(err)
		return Bot{}, err
	}

	// check if the slave has any bots
	if len(svent.bots) == 0 {
		err := fmt.Errorf(
			"%w, slave %v requested for a bot but has no bots assigned",
			errNoBots,
			svent.slvMailId,
		)
		bv.logger.Error(err)
		return Bot{}, err
	}

	svent.lastUsedBot = (svent.lastUsedBot + 1) % len(svent.bots)
	nextBot := svent.bots[svent.lastUsedBot]
	svent.lastUsedBot = (svent.lastUsedBot + 1) % len(svent.bots)

	bv.inventory[slaveID] = svent

	return nextBot, nil
}
