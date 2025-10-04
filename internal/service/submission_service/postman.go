package submission_service

import (
	"github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

func (p *postman) start() {
	p.mailBox = make(chan mail, 50)
	p.mailClients = make(map[mailID]mailClient)
	p.logger = logrus.WithFields(
		logrus.Fields{
			"from": "postman",
		},
	)

	go p.deliverMails()
	p.logger.Info("postman started delivering mails")
}

func (p *postman) deliverMails() {
	for ml := range p.mailBox {
		// handle mails to postman
		if ml.to == mailPostman {
			p.handlePostmanMails(ml)
			continue
		}

		// get the mail client from clients map
		p.Lock()
		client, ok := p.mailClients[ml.to]
		p.Unlock()

		if !ok {
			p.logger.Errorf(
				"client with given mailID doesn't exist. cannot deliver mail %v",
				ml,
			)
			if ml.from == mailPostman {
				continue
			}
			// inform the sender
			invMail := mail{
				from: mailPostman,
				to:   ml.from,
				body: invalidMailClient(ml.to),
				// this is by far the most important mail used to avoid more dependency on invalid clients.
				// also mostly its very easy to process this mail
				priority: 20,
			}
			p.postMail(invMail)
			continue
		}
		go client.recieveMail(ml)
	}
}

func (p *postman) handlePostmanMails(pmail mail) {
	switch body := pmail.body.(type) {
	case unregisterMailClient:
		p.Lock()
		defer p.Unlock()

		clientMailID := mailID(body)

		// check if its a valid mail client
		_, ok := p.mailClients[clientMailID]
		if !ok {
			p.logger.Errorf(
				"client %v has request to unregister invalid client %v",
				pmail.from, clientMailID,
			)
			return
		}

		// delete
		delete(p.mailClients, clientMailID)
		p.logger.Debugf(
			"client %v has been unregistered as requested by %v",
			clientMailID, pmail.from,
		)
	default:
		p.logger.Errorf("ignoring invalid mail %v", pmail)
	}
}

// NOTE: all the communication must happen through mails only. However this method
// is called directly instead of mails to avoid extra complexity where the registree clients need
// to be kept in a seperate wating list, handle duplicate registering alerts, handle
// registration alerts from postman and so on. Given a small number of events calling this function
// and also the low cost of this functions, its better to call this method directly as it ensure to use
// proper locking mechanism to avoid race conditions.
func (p *postman) RegisterMailClient(id mailID, client mailClient) error {
	p.Lock()
	defer p.Unlock()

	// try to check if client with given mail id exist. if so, return an error
	if _, ok := p.mailClients[id]; ok {
		return flux_errors.ErrEntityAlreadyExist
	}
	p.mailClients[id] = client

	return nil
}

func (p *postman) postMail(mail mail) {
	p.mailBox <- mail
}
