package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a webhook subscription is not found.
var ErrNotFound = errors.New("webhook subscription not found")

// Repository provides persistence for webhook subscriptions and deliveries.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new webhook Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new webhook subscription.
func (r *Repository) Create(ctx context.Context, sub *WebhookSubscription) error {
	sub.ID = uuid.New()
	sub.CreatedAt = time.Now().UTC()
	sub.Active = true

	query := `INSERT INTO webhook_subscriptions (id, user_id, url, events, secret, active, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(ctx, query,
		sub.ID, sub.UserID, sub.URL, sub.Events, sub.Secret, sub.Active, sub.CreatedAt,
	)
	return err
}

// GetByID retrieves a subscription by ID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*WebhookSubscription, error) {
	query := `SELECT id, user_id, url, events, secret, active, created_at
	          FROM webhook_subscriptions WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var sub WebhookSubscription
	if err := row.Scan(&sub.ID, &sub.UserID, &sub.URL, &sub.Events, &sub.Secret, &sub.Active, &sub.CreatedAt); err != nil {
		return nil, ErrNotFound
	}
	return &sub, nil
}

// ListByUser returns all subscriptions for a user.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*WebhookSubscription, error) {
	query := `SELECT id, user_id, url, events, secret, active, created_at
	          FROM webhook_subscriptions WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*WebhookSubscription
	for rows.Next() {
		var sub WebhookSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.URL, &sub.Events, &sub.Secret, &sub.Active, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, rows.Err()
}

// ListByEvent returns all active subscriptions listening for a given event type.
func (r *Repository) ListByEvent(ctx context.Context, eventType string) ([]*WebhookSubscription, error) {
	query := `SELECT id, user_id, url, events, secret, active, created_at
	          FROM webhook_subscriptions
	          WHERE active = true AND $1 = ANY(events)
	          ORDER BY created_at`
	rows, err := r.db.Query(ctx, query, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*WebhookSubscription
	for rows.Next() {
		var sub WebhookSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.URL, &sub.Events, &sub.Secret, &sub.Active, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, rows.Err()
}

// Delete removes a subscription.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM webhook_subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordDelivery records a webhook delivery attempt.
func (r *Repository) RecordDelivery(ctx context.Context, d *WebhookDelivery) error {
	d.ID = uuid.New()
	d.DeliveredAt = time.Now().UTC()

	payload, _ := json.Marshal(map[string]string{})
	query := `INSERT INTO webhook_deliveries (id, subscription_id, event_type, payload, status_code, attempt, success, error_message, delivered_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.Exec(ctx, query,
		d.ID, d.SubscriptionID, d.EventType, payload,
		d.StatusCode, d.Attempt, d.Success, d.ErrorMessage, d.DeliveredAt,
	)
	return err
}
