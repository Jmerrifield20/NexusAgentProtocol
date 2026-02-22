package email

import (
	"context"

	"go.uber.org/zap"
)

// NoopSender logs emails to zap instead of delivering them.
// Use in development or when SMTP is not configured.
type NoopSender struct {
	logger *zap.Logger
}

// NewNoopSender creates a NoopSender backed by the given logger.
func NewNoopSender(logger *zap.Logger) *NoopSender {
	return &NoopSender{logger: logger}
}

// Send logs the email and returns nil.
func (n *NoopSender) Send(_ context.Context, to, subject, body string) error {
	n.logger.Info("email (noop — not sent)",
		zap.String("to", to),
		zap.String("subject", subject),
		zap.String("body", body),
	)
	return nil
}
