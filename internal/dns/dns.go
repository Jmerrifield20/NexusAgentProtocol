package dns

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

// Challenge holds the state for a DNS-01 domain ownership challenge.
type Challenge struct {
	Domain    string
	Token     string // random token to be placed in DNS TXT record
	TXTRecord string // full expected TXT record value
	ExpiresAt time.Time
}

const txtRecordPrefix = "_nexus-agent-challenge."

// NewChallenge generates a DNS-01 challenge for the given domain.
func NewChallenge(domain string) (*Challenge, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &Challenge{
		Domain:    domain,
		Token:     token,
		TXTRecord: "nexus-agent-challenge=" + token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, nil
}

// TXTHost returns the DNS hostname where the TXT record must be placed.
func (c *Challenge) TXTHost() string {
	return TXTHost(c.Domain)
}

// TXTHost returns the DNS hostname where the TXT record must be placed for domain.
// This is the package-level helper used when reconstructing a challenge from storage.
func TXTHost(domain string) string {
	return txtRecordPrefix + strings.TrimSuffix(domain, ".")
}

// Verify checks that the TXT record has been published in DNS.
func (c *Challenge) Verify(ctx context.Context) error {
	if time.Now().After(c.ExpiresAt) {
		return fmt.Errorf("challenge expired at %s", c.ExpiresAt.Format(time.RFC3339))
	}

	host := c.TXTHost()
	resolver := &net.Resolver{}

	txts, err := resolver.LookupTXT(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}

	for _, txt := range txts {
		if txt == c.TXTRecord {
			return nil // verified
		}
	}

	return fmt.Errorf("TXT record not found at %s; expected %q", host, c.TXTRecord)
}

// generateToken produces a cryptographically random URL-safe token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
