package email

import (
	"fmt"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

var (
	emailChan chan emailJob
	once      sync.Once
)

func StartEmailWorkers(numWorkers int) {
	// using sync once to ensure that this happens only once even if the function is called multiple times
	once.Do(func() {
		emailChan = make(chan emailJob, defaultEmailChannelCapacity)
		log.Infof("starting %d email workers", numWorkers)
		for i := range numWorkers {
			go worker(i + 1)
		}
	})
}

/*
	Worker listens through the emailChan in an infinite loop
	Once it gets a mail, it sends via the dialer
	If it doesn't get a mail for 30 seconds straight, it closes the sender
*/

func worker(id int) {
	dailer, err := getDialer()
	workerLogger := log.WithField("worker", id)
	if err != nil {
		workerLogger.Error("unable to get dailer. email worker stopped")
		return
	}
	var s gomail.SendCloser // used to send the actual mail
	open := false
	for {
		// select statement waits until any one of the channel
		// gets any input. then it executes the specific part and then exits
		select {
		case job, ok := <-emailChan:
			if !ok {
				// channel is closed, stop the worker
				workerLogger.Info("email channel is closed. email worker stopped")
				return
			}

			if !open {
				// email sendcloser was closed previously
				workerLogger.Info("opening a new email sender")
				if s, err = dailer.Dial(); err != nil {
					workerLogger.Errorf("cannot open an email sendcloser, %v, unable to send mail", err)
					continue
				}
				open = true
			}

			// create a custom logger
			mailLogger := workerLogger.WithFields(
				log.Fields{
					"recipients": job.to,
					"purpose":    job.purpose,
				},
			)

			// construct *gomail.Message from job
			mail := constructMail(job)
			if sendErr := gomail.Send(s, mail); sendErr != nil {
				// The connection might be bad, so we'll close and open a new one.
				mailLogger.Errorf("failed to send mail, will attempt to redial: %v", sendErr)
				if s != nil {
					if closeErr := s.Close(); closeErr != nil {
						mailLogger.Errorf("error closing connection after send failure: %v", closeErr)
					}
				}
				open = false
			} else {
				mailLogger.Info("mail sent successfully")
			}

		// close the sender if no mail is sent in the past 30 seconds
		case <-time.After(30 * time.Second):
			if open {
				workerLogger.Info("no mail recieved in past 30 seconds. closing the email sender")
				if err := s.Close(); err != nil {
					workerLogger.Error("unable to close the sender")
				}
				open = false
			}
		}
	}
}

func constructMail(job emailJob) *gomail.Message {
	mail := gomail.NewMessage()
	mail.SetHeader(KeyEmailFrom, job.from)
	mail.SetHeader(KeyEmailTo, job.to...)
	mail.SetHeader(KeyEmailSubject, job.subject)
	mail.SetBody(string(job.bodyType), job.body)
	return mail
}

func getDialer() (*gomail.Dialer, error) {
	smtpServer := KeyEmailSMTPServer
	smtpPort := KeyEmailSMTPPort
	fromMail := os.Getenv(KeyEmailSender)
	if fromMail == "" {
		return nil, fmt.Errorf("sender email (%s) not found in environment variables", KeyEmailSender)
	}

	password := os.Getenv(KeyEmailSenderPassword)
	if password == "" {
		return nil, fmt.Errorf("sender password (%s) not found in environment variables", KeyEmailSenderPassword)
	}

	return gomail.NewDialer(smtpServer, smtpPort, fromMail, password), nil
}
