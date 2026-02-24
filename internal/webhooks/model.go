package webhooks

import (
	"time"

	"github.com/google/uuid"
)

// Event types dispatched by the system.
const (
	EventAgentRegistered    = "agent.registered"
	EventAgentActivated     = "agent.activated"
	EventAgentRevoked       = "agent.revoked"
	EventAgentSuspended     = "agent.suspended"
	EventAgentDeprecated    = "agent.deprecated"
	EventAgentHealthDegraded = "agent.health_degraded"
)

// WebhookSubscription represents a user's subscription to webhook events.
type WebhookSubscription struct {
	ID        uuid.UUID `json:"id"         db:"id"`
	UserID    uuid.UUID `json:"user_id"    db:"user_id"`
	URL       string    `json:"url"        db:"url"`
	Events    []string  `json:"events"     db:"events"`
	Secret    string    `json:"-"          db:"secret"` // never returned in API responses
	Active    bool      `json:"active"     db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// WebhookEvent is dispatched to matching subscriptions.
type WebhookEvent struct {
	Type      string            `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Payload   map[string]string `json:"payload"`
}

// WebhookDelivery records the outcome of a single delivery attempt.
type WebhookDelivery struct {
	ID             uuid.UUID `json:"id"              db:"id"`
	SubscriptionID uuid.UUID `json:"subscription_id" db:"subscription_id"`
	EventType      string    `json:"event_type"      db:"event_type"`
	StatusCode     int       `json:"status_code"     db:"status_code"`
	Attempt        int       `json:"attempt"         db:"attempt"`
	Success        bool      `json:"success"         db:"success"`
	ErrorMessage   string    `json:"error_message"   db:"error_message"`
	DeliveredAt    time.Time `json:"delivered_at"     db:"delivered_at"`
}

// CreateSubscriptionRequest is the payload for creating a webhook subscription.
type CreateSubscriptionRequest struct {
	URL    string   `json:"url"    binding:"required,url"`
	Events []string `json:"events" binding:"required"`
}
