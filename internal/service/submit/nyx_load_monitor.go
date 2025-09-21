package submission_service

import (
	"fmt"
	"slices"
	"time"

	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (mnr *nyxLdMnr) startLoadMonitor() error {
	mnr.pq = NewPriorityQueue[subTimeStamp](lane.MINPQ)
	mnr.reqTimeStamps = make(chan time.Time, 20)
	mnr.loadWindow = make([]int32, 0)
	mnr.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailNyxLdMnr,
		},
	)

	if mnr.postman == nil {
		err := fmt.Errorf(
			"%w, non-nil postman expected",
			flux_errors.ErrInternal,
		)
		mnr.logger.Error(err)
		return err
	}

	go mnr.monitorLoad()

	mnr.logger.Debugf("load monitor started successfully")
	return nil
}

// monitors load every 30 seconds
func (mnr *nyxLdMnr) monitorLoad() {
	// Unix() gives int64 which on converting to int will on 32 bit system overflow
	// so we will consider all the timestamps from jan 1 of 2025
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

	for {
		// empty the channel
	drain:
		for {
			select {
			case stamp := <-mnr.reqTimeStamps:
				cur := stamp.UTC().Unix() - base
				mnr.pq.Add(subTimeStamp(cur))
			default:
				break drain
			}
		}

		// get timestamp of past 5th minute
		past5 := int(time.Now().Add(time.Minute*-5).UTC().Unix() - base)
		for top, ok := mnr.pq.Peek(); ok; {
			if past5 > int(top) {
				break
			}
		}

		// update current load
		curLoad := mnr.pq.Size()
		if len(mnr.loadWindow) < reqLoadWindowSize {
			mnr.loadWindow = append(mnr.loadWindow, int32(curLoad))
		} else {
			leftShift(mnr.loadWindow)
			mnr.loadWindow[reqLoadWindowSize-1] = int32(curLoad)
		}

		// inform
		loadWindow := slices.Clone(mnr.loadWindow) // make a copy to avoid races
		mnr.logger.Debugf("current load: %v", loadWindow)
		reportMail := mail{
			from:     mailNyxLdMnr,
			to:       mailNyxMaster,
			body:     loadWindow,
			priority: prNyxMstLoadReport,
		}
		mnr.postman.postMail(reportMail)

		time.Sleep(time.Second * 30)
	}
}

func (mnr *nyxLdMnr) subAlert(t time.Time) {
	mnr.reqTimeStamps <- t
}
