package email

import (
	"context"
	"errors"
	"os"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/database"
	"github.com/tcp_snm/flux/internal/flux_errors"
	"github.com/tcp_snm/flux/internal/service/user_service"
)

type EmailPurpose string
type EmailBodyType string

var (
	errMsgs = make(map[string]map[string]string)
)

const (
	KeyEmailSender                            = "SENDER_EMAIL"
	KeyEmailSenderPassword                    = "SENDER_EMAIL_PASSWORD"
	KeyEmailSMTPServer                        = "smtp.gmail.com"
	KeyEmailSMTPPort                          = 587
	KeyEmailFrom                              = "From"
	KeyEmailTo                                = "To"
	KeyEmailSubject                           = "Subject"
	KeyEmailBodyPlain           EmailBodyType = "text/plain"
	PurposeEmailPasswordReset   EmailPurpose  = "reset_password"
	PurposeEmailSignUp          EmailPurpose  = "sign_up"
	PurposeBotNotWorkingAlert   EmailPurpose  = "bot not working"
	defaultEmailChannelCapacity               = 100
)

type EmailRequest struct {
	To       []string
	Subject  string
	Body     string
	BodyType EmailBodyType
	Purpose  EmailPurpose
}

type emailJob struct {
	EmailRequest
	from string
}

type EmailService struct {
	DB     *database.Queries
	logger *logrus.Entry
}

func (e *EmailService) Start() {
	if e.DB == nil {
		panic("email service expects non-nil db")
	}

	e.logger = logrus.WithField("from", "email service")
}

// this function can be made better by accepting all arguments as EmailRequest
// but already its was being used by many other services. Its left intact for backward compatibility
func NewMail(
	ctx context.Context,
	subject string,
	body string,
	bodyType EmailBodyType,
	purpose EmailPurpose,
	to ...string,
) error {
	fromMail := os.Getenv(KeyEmailSender)
	if fromMail == "" {
		log.Error("sender email is not configured")
		return flux_errors.ErrEmailServiceStopped
	}
	job := emailJob{
		from: fromMail,
		EmailRequest: EmailRequest{
			To:       to,
			Subject:  subject,
			Body:     body,
			BodyType: bodyType,
			Purpose:  purpose,
		},
	}
	// when all the workers are dead, it shouldn't block indefinetely
	select {
	case <-ctx.Done():
		// The context was canceled or timed out, so we return an error.
		// This prevents the application from hanging indefinitely.
		log.Errorf("email job cancelled: %v", ctx.Err())
		return errors.Join(flux_errors.ErrEmailServiceStopped, ctx.Err())

	case emailChan <- job:
		// A worker was available, and the job was sent successfully.
		return nil
	}
}

func (e *EmailService) MailManagers(ctx context.Context, req EmailRequest) error {
	managerMails, err := e.getManagerEmails(ctx)
	if err != nil {
		return err
	}
	NewMail(
		ctx,
		req.Subject,
		req.Body,
		req.BodyType,
		req.Purpose,
		managerMails...,
	)
	e.logger.Infof("sent mail to managers for %v purpose", req.Purpose)
	return nil
}

func (e *EmailService) getManagerEmails(ctx context.Context) ([]string, error) {
	emails, err := e.DB.GetManagerMailIDs(ctx, user_service.RoleManager)
	if err != nil {
		err = flux_errors.HandleDBErrors(
			err,
			errMsgs,
			"cannot get manager mails from db",
		)
		return nil, err
	}

	return emails, nil
}
