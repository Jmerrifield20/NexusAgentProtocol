package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	internaldns "github.com/jmerrifield20/NexusAgentProtocol/internal/dns"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/model"
	"github.com/jmerrifield20/NexusAgentProtocol/internal/registry/repository"
	"go.uber.org/zap"
)

// challengeStore is the storage interface required by DNSChallengeService.
// *repository.DNSChallengeRepository satisfies this interface.
type challengeStore interface {
	Create(ctx context.Context, ch *model.DNSChallenge) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.DNSChallenge, error)
	MarkVerified(ctx context.Context, id uuid.UUID) error
	FindVerifiedByDomain(ctx context.Context, domain string) (*model.DNSChallenge, error)
	DeleteExpired(ctx context.Context) (int64, error)
}

// verifyFn is a function that performs the actual DNS TXT lookup for a challenge.
// In production this is challenge.Verify; in tests it can be stubbed.
type verifyFn func(ctx context.Context, ch *internaldns.Challenge) error

// DNSChallengeService manages DNS-01 domain ownership challenges.
type DNSChallengeService struct {
	store    challengeStore
	verify   verifyFn
	logger   *zap.Logger
}

// NewDNSChallengeService creates a DNSChallengeService.
// The store is typically *repository.DNSChallengeRepository.
// Pass nil for verify to use real DNS lookups.
func NewDNSChallengeService(store challengeStore, verify verifyFn, logger *zap.Logger) *DNSChallengeService {
	if verify == nil {
		verify = func(ctx context.Context, ch *internaldns.Challenge) error {
			return ch.Verify(ctx)
		}
	}
	return &DNSChallengeService{store: store, verify: verify, logger: logger}
}

// StartChallenge issues a new DNS-01 challenge for the given domain.
// The caller must instruct the domain owner to publish the returned TXT record
// at challenge.TXTHost before calling VerifyChallenge.
func (s *DNSChallengeService) StartChallenge(ctx context.Context, domain string) (*model.DNSChallenge, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain must not be empty")
	}

	raw, err := internaldns.NewChallenge(domain)
	if err != nil {
		return nil, fmt.Errorf("generate challenge: %w", err)
	}

	ch := &model.DNSChallenge{
		Domain:    raw.Domain,
		Token:     raw.Token,
		TXTRecord: raw.TXTRecord,
		TXTHost:   raw.TXTHost(),
		ExpiresAt: raw.ExpiresAt,
	}

	if err := s.store.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("persist challenge: %w", err)
	}

	s.logger.Info("DNS challenge started",
		zap.String("domain", domain),
		zap.String("txt_host", ch.TXTHost),
		zap.String("txt_record", ch.TXTRecord),
		zap.Time("expires_at", ch.ExpiresAt),
	)
	return ch, nil
}

// GetChallenge returns the current state of a challenge by ID.
func (s *DNSChallengeService) GetChallenge(ctx context.Context, id uuid.UUID) (*model.DNSChallenge, error) {
	ch, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrChallengeNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("get challenge: %w", err)
	}
	// Populate the computed TXTHost field.
	ch.TXTHost = internaldns.TXTHost(ch.Domain)
	return ch, nil
}

// VerifyChallenge performs the DNS TXT lookup for the challenge identified by id.
// On success the challenge is marked verified in the database.
func (s *DNSChallengeService) VerifyChallenge(ctx context.Context, id uuid.UUID) (*model.DNSChallenge, error) {
	ch, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrChallengeNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("get challenge: %w", err)
	}

	if ch.Verified {
		ch.TXTHost = internaldns.TXTHost(ch.Domain)
		return ch, nil // idempotent
	}

	if time.Now().After(ch.ExpiresAt) {
		return nil, ErrChallengeExpired
	}

	// Reconstruct the dns.Challenge for verification.
	raw := &internaldns.Challenge{
		Domain:    ch.Domain,
		Token:     ch.Token,
		TXTRecord: ch.TXTRecord,
		ExpiresAt: ch.ExpiresAt,
	}

	if err := s.verify(ctx, raw); err != nil {
		s.logger.Info("DNS challenge verification failed",
			zap.String("domain", ch.Domain),
			zap.String("id", id.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %s", ErrVerificationFailed, err.Error())
	}

	if err := s.store.MarkVerified(ctx, id); err != nil {
		return nil, fmt.Errorf("mark verified: %w", err)
	}
	ch.Verified = true
	ch.TXTHost = internaldns.TXTHost(ch.Domain)

	s.logger.Info("DNS challenge verified",
		zap.String("domain", ch.Domain),
		zap.String("id", id.String()),
	)
	return ch, nil
}

// IsDomainVerified returns true if there is at least one verified DNS challenge
// for the domain. A nil error with false means no verified challenge exists.
func (s *DNSChallengeService) IsDomainVerified(ctx context.Context, domain string) (bool, error) {
	_, err := s.store.FindVerifiedByDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, repository.ErrChallengeNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check domain verification: %w", err)
	}
	return true, nil
}

// DeleteExpired removes all unverified challenges that have passed their expiry.
// Returns the number of rows removed. Safe to call from a background goroutine.
func (s *DNSChallengeService) DeleteExpired(ctx context.Context) (int64, error) {
	n, err := s.store.DeleteExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired challenges: %w", err)
	}
	if n > 0 {
		s.logger.Info("pruned expired DNS challenges", zap.Int64("count", n))
	}
	return n, nil
}

// Sentinel errors for the DNS challenge service.
var (
	ErrChallengeNotFound  = errors.New("dns challenge not found")
	ErrChallengeExpired   = errors.New("dns challenge has expired; start a new one")
	ErrVerificationFailed = errors.New("dns verification failed")
)
