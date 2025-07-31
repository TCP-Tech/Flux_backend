package email

import (
	"context"
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tcp_snm/flux/internal/flux_errors"
)

type EmailPurpose string
type EmailBodyType string

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
	defaultEmailChannelCapacity               = 100
)

type emailJob struct {
	from     string
	to       []string
	subject  string
	body     string
	bodyType EmailBodyType
	purpose  EmailPurpose
}

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
		from:     fromMail,
		to:       to,
		subject:  subject,
		body:     body,
		bodyType: bodyType,
		purpose:  purpose,
	}
	// when all the workers it shouldn't block indefinetely
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
