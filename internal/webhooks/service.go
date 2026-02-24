package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MetricsRecorder is an optional callback for recording delivery outcomes.
type MetricsRecorder func(success bool)

// Service manages webhook subscriptions and event dispatching.
type Service struct {
	repo       *Repository
	httpClient *http.Client
	onMetrics  MetricsRecorder
	logger     *zap.Logger
}

// NewService creates a new webhook Service.
func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{
		repo:       repo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// SetMetricsRecorder configures the metrics callback.
func (s *Service) SetMetricsRecorder(fn MetricsRecorder) {
	s.onMetrics = fn
}

// Subscribe creates a new webhook subscription with a generated HMAC secret.
func (s *Service) Subscribe(ctx context.Context, userID uuid.UUID, req *CreateSubscriptionRequest) (*WebhookSubscription, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	sub := &WebhookSubscription{
		UserID: userID,
		URL:    req.URL,
		Events: req.Events,
		Secret: secret,
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	return sub, nil
}

// Unsubscribe deletes a subscription, checking ownership.
func (s *Service) Unsubscribe(ctx context.Context, userID, subID uuid.UUID) error {
	sub, err := s.repo.GetByID(ctx, subID)
	if err != nil {
		return err
	}
	if sub.UserID != userID {
		return fmt.Errorf("not authorized to delete this subscription")
	}
	return s.repo.Delete(ctx, subID)
}

// ListByUser returns all subscriptions for a user.
func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID) ([]*WebhookSubscription, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Dispatch fans out a webhook event to all matching subscriptions.
// Implements the service.WebhookDispatcher interface.
func (s *Service) Dispatch(ctx context.Context, eventType string, payload map[string]string) {
	subs, err := s.repo.ListByEvent(ctx, eventType)
	if err != nil {
		s.logger.Error("webhook: list subscribers", zap.Error(err))
		return
	}

	event := WebhookEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}

	for _, sub := range subs {
		go s.deliver(ctx, sub, event)
	}
}

// deliver sends the event to a single subscription with retries.
func (s *Service) deliver(ctx context.Context, sub *WebhookSubscription, event WebhookEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("webhook: marshal event", zap.Error(err))
		return
	}

	signature := signPayload(body, sub.Secret)

	// Retry with exponential backoff: 1s, 5s, 25s.
	delays := []time.Duration{0, 1 * time.Second, 5 * time.Second, 25 * time.Second}

	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			time.Sleep(delays[attempt])
		}

		success, statusCode, errMsg := s.doDelivery(ctx, sub.URL, body, signature)

		delivery := &WebhookDelivery{
			SubscriptionID: sub.ID,
			EventType:      event.Type,
			StatusCode:     statusCode,
			Attempt:        attempt,
			Success:        success,
			ErrorMessage:   errMsg,
		}
		if recordErr := s.repo.RecordDelivery(ctx, delivery); recordErr != nil {
			s.logger.Warn("webhook: record delivery", zap.Error(recordErr))
		}

		if s.onMetrics != nil {
			s.onMetrics(success)
		}

		if success {
			return
		}

		s.logger.Warn("webhook: delivery failed",
			zap.String("url", sub.URL),
			zap.Int("attempt", attempt),
			zap.String("error", errMsg),
		)
	}
}

// doDelivery performs a single HTTP POST delivery.
func (s *Service) doDelivery(ctx context.Context, url string, body []byte, signature string) (bool, int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NAP-Signature", signature)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, 0, err.Error()
	}
	defer resp.Body.Close()
	io.ReadAll(io.LimitReader(resp.Body, 1024)) //nolint:errcheck

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	errMsg := ""
	if !success {
		errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return success, resp.StatusCode, errMsg
}

// signPayload computes an HMAC-SHA256 signature.
func signPayload(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// generateSecret creates a random 32-byte hex-encoded secret.
func generateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
