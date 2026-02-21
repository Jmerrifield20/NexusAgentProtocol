package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexus-protocol/nexus/internal/registry/model"
)

// ErrChallengeNotFound is returned when a DNS challenge is not found.
var ErrChallengeNotFound = errors.New("dns challenge not found")

// DNSChallengeRepository provides persistence for DNS-01 challenges.
type DNSChallengeRepository struct {
	db *pgxpool.Pool
}

// NewDNSChallengeRepository creates a new DNSChallengeRepository.
func NewDNSChallengeRepository(db *pgxpool.Pool) *DNSChallengeRepository {
	return &DNSChallengeRepository{db: db}
}

// Create inserts a new DNS challenge record.
func (r *DNSChallengeRepository) Create(ctx context.Context, ch *model.DNSChallenge) error {
	ch.ID = uuid.New()
	ch.CreatedAt = time.Now().UTC()

	_, err := r.db.Exec(ctx,
		`INSERT INTO dns_challenges (id, domain, token, txt_record, verified, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, false, $5, $6)`,
		ch.ID, ch.Domain, ch.Token, ch.TXTRecord, ch.CreatedAt, ch.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert dns challenge: %w", err)
	}
	return nil
}

// GetByID returns a single DNS challenge by its UUID.
func (r *DNSChallengeRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.DNSChallenge, error) {
	ch := &model.DNSChallenge{}
	err := r.db.QueryRow(ctx,
		`SELECT id, domain, token, txt_record, verified, created_at, expires_at
		 FROM dns_challenges WHERE id = $1`, id,
	).Scan(&ch.ID, &ch.Domain, &ch.Token, &ch.TXTRecord, &ch.Verified, &ch.CreatedAt, &ch.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("get dns challenge: %w", err)
	}
	return ch, nil
}

// MarkVerified sets verified=true on the given challenge.
func (r *DNSChallengeRepository) MarkVerified(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE dns_challenges SET verified = true WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("mark challenge verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

// FindVerifiedByDomain returns the most recent verified (and non-expired) challenge
// for the given domain, or ErrChallengeNotFound if none exists.
func (r *DNSChallengeRepository) FindVerifiedByDomain(ctx context.Context, domain string) (*model.DNSChallenge, error) {
	ch := &model.DNSChallenge{}
	err := r.db.QueryRow(ctx,
		`SELECT id, domain, token, txt_record, verified, created_at, expires_at
		 FROM dns_challenges
		 WHERE domain = $1 AND verified = true
		 ORDER BY created_at DESC
		 LIMIT 1`, domain,
	).Scan(&ch.ID, &ch.Domain, &ch.Token, &ch.TXTRecord, &ch.Verified, &ch.CreatedAt, &ch.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("find verified challenge: %w", err)
	}
	return ch, nil
}

// DeleteExpired removes all challenges whose expires_at is in the past and
// which have not been verified. Returns the number of rows deleted.
func (r *DNSChallengeRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM dns_challenges WHERE expires_at < now() AND verified = false`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired challenges: %w", err)
	}
	return tag.RowsAffected(), nil
}
