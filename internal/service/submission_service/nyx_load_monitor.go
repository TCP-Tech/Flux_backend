package submission_service

import (
	"math"
	"time"

	"github.com/oleiade/lane"
	"github.com/sirupsen/logrus"
)

func (mnr *nyxLdMnr) start() {
	mnr.subReqT = NewPriorityQueue[subTimeStamp](lane.MINPQ)
	mnr.mailBox = make(chan mail, 20)
	mnr.logger = logrus.WithFields(
		logrus.Fields{
			"from": mailNyxLdMnr,
		},
	)
	mnr.subTChan = make(chan subTAlert, 10)

	if mnr.postman == nil {
		panic("nyx load manager expects non-nil postman")
	}

	go mnr.processMails()
	go mnr.monitorLoad()
	go mnr.monitorSubTime()
	go mnr.reportLoad()

	mnr.logger.Debugf("load monitor started successfully")
}

func (mnr *nyxLdMnr) processMails() {
	for mail := range mnr.mailBox {
		switch body := mail.body.(type) {
		case subAlert:
			mnr.subReqT.Add(
				subTimeStamp(time.Time(body).UTC().Unix() - baseTimeStamp.Unix()),
			)
		case subTAlert:
			mnr.subTChan <- body
		default:
			mnr.logger.WithField("mail", mail).Warn(
				"recieved unknown body in mail",
			)
		}
	}
}

func (mnr *nyxLdMnr) getMailID() mailID {
	return mailNyxLdMnr
}

func (mnr *nyxLdMnr) recieveMail(ml mail) {
	mnr.mailBox <- ml
}

// monitors load every 60 seconds
func (mnr *nyxLdMnr) monitorLoad() {
	for {
		// Get the timestamp for one minute ago
		oneMinAgo := int(time.Now().Add(time.Minute*-1).UTC().Unix() - baseTimeStamp.Unix())

		// Keep looping as long as there are old items to remove.
		for {
			top, ok := mnr.subReqT.Peek()
			if !ok {
				// The queue is empty, so we're done.
				break
			}

			if int(top) > oneMinAgo {
				// The oldest item is NEWER than one minute ago.
				// Stop cleaning and keep this item in the queue.
				break
			}

			// If we're here, the item is old, so remove it and loop again.
			mnr.subReqT.Pop()
		}

		// Now calculate the load with only recent items left in the queue.
		curLoad := mnr.subReqT.Size()

		// Update average load
		mnr.Lock()
		mnr.avgLoad = 0.45*mnr.avgLoad + 0.55*float64(curLoad)
		mnr.logger.Debugf("current averageLoad: %.3f", mnr.avgLoad)
		mnr.Unlock()

		time.Sleep(time.Second * 60)
	}

}

/*
	Time taken by nyx slave to submit each solution varies according to the network latency,
	codeforces site response along with some other external factors.

	Monitoring time taken for each submission helps us get a neat estimate
	of what is the expected time for next submission would be.

	It will also help us decide how many number of slaves we should currently
	keep in inventory to process submissions fairly quickly.

	We use exponential weighted moving average or EWMA to get the current estimate each
	time a new sample arrives. Our average also depends on the past average so that it
	would be less sensitive to noisy samples (eg., if its too quick or too late)

	Also the past average decays overtime. If the past average was calculate not too long ago,
	it will have more importance. Its influence on current average will drop exponentially.

	The mathematics of calculation is as follows:
	1. alpha = exp(-delta/tou)
	2. currentAvg = alpha * pastAverage + (1 - alpha) * latestSample where,
	3. delta = time interval between latest sampling and time at which previous average was calculated (+ve)
	4. tou = a constant which helps us control the sensitivity of the current average towards latest samples
	   if tou is large it is less sensitive i.e., its more smooth.
	   if tou is small, its more sensitive to latest samples and noise
	   tou = 15 => if past sample was taken 15 seconds ago, then its importance decays by e => 37%
*/

func (mnr *nyxLdMnr) monitorSubTime() {
	var tou time.Duration = 15 * time.Second
	for sample := range mnr.subTChan {
		mnr.Lock()

		// calculate delta
		var delta time.Duration
		if sample.sampleTime.After(mnr.avgSubT.previousSampleTime) {
			delta = sample.sampleTime.Sub(mnr.avgSubT.previousSampleTime)
			mnr.avgSubT.previousSampleTime = sample.sampleTime
		} else {
			// assume a small delay between consecutive samples as a fallback
			delta = 5 * time.Millisecond
		}

		// calculate alpha
		alpha := math.Exp(-delta.Seconds() / tou.Seconds())

		// calculate current average
		currentAvg := alpha*mnr.avgSubT.previousAverage.Seconds() + (1-alpha)*sample.duration.Seconds()

		// update
		currentAvgSecs := time.Duration(currentAvg * float64(time.Second))
		mnr.avgSubT.previousAverage = currentAvgSecs
		mnr.logger.Debugf("latest submission time average: %v", currentAvgSecs)

		mnr.Unlock()
	}
}

// reports latest load to master every 30 seconds
func (mnr *nyxLdMnr) reportLoad() {
	for {
		time.Sleep(time.Second * 30)

		var load loadReport
		mnr.Lock()
		load = loadReport{
			avgLoad: mnr.avgLoad,
			avgSubT: mnr.avgSubT.previousAverage,
		}
		mnr.Unlock()

		// inform
		mnr.postman.postMail(
			mail{
				from:     mailNyxLdMnr,
				to:       mailNyxMaster,
				body:     load,
				priority: prNyxMstLoadReport,
			},
		)
	}
}
