package dns_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jmerrifield20/NexusAgentProtocol/internal/dns"
)

func TestNewChallenge(t *testing.T) {
	ch, err := dns.NewChallenge("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Domain != "example.com" {
		t.Errorf("Domain: got %q, want %q", ch.Domain, "example.com")
	}
	if ch.Token == "" {
		t.Error("Token must not be empty")
	}
	if !strings.HasPrefix(ch.TXTRecord, "nexus-agent-challenge=") {
		t.Errorf("TXTRecord format unexpected: %q", ch.TXTRecord)
	}
	if ch.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt should be in the future")
	}
}

func TestChallenge_TXTHost(t *testing.T) {
	ch, _ := dns.NewChallenge("example.com")
	host := ch.TXTHost()
	if !strings.HasSuffix(host, "example.com") {
		t.Errorf("TXTHost should end with domain, got %q", host)
	}
	if !strings.HasPrefix(host, "_nexus-agent-challenge.") {
		t.Errorf("TXTHost should start with challenge prefix, got %q", host)
	}
}
