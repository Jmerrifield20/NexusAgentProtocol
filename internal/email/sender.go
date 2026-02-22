package email

import "context"

// EmailSender delivers transactional email.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}
