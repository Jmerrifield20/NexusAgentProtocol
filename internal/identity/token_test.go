package identity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/internal/identity"
)

func newTestTokenIssuer(t *testing.T) *identity.TokenIssuer {
	t.Helper()
	ca := newTestCA(t) // reuse CA helper from ca_test.go
	return identity.NewTokenIssuer(ca.Key(), "https://registry.nexusagentprotocol.com", time.Hour)
}

func TestTokenIssuer_Issue(t *testing.T) {
	ti := newTestTokenIssuer(t)

	token, err := ti.Issue("agent://nexusagentprotocol.com/finance/taxes/agent_abc", []string{"agent:resolve", "agent:call"})
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3-part JWT, got %d parts", len(parts))
	}
}

func TestTokenIssuer_Verify_valid(t *testing.T) {
	ti := newTestTokenIssuer(t)
	agentURI := "agent://nexusagentprotocol.com/assistant/agent_xyz"
	scopes := []string{"agent:resolve"}

	token, err := ti.Issue(agentURI, scopes)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ti.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	if claims.AgentURI != agentURI {
		t.Errorf("AgentURI: got %q, want %q", claims.AgentURI, agentURI)
	}
	if claims.Subject != agentURI {
		t.Errorf("Subject: got %q, want %q", claims.Subject, agentURI)
	}
	if len(claims.Scopes) != 1 || claims.Scopes[0] != "agent:resolve" {
		t.Errorf("Scopes: got %v, want [agent:resolve]", claims.Scopes)
	}
}

func TestTokenIssuer_Verify_expired(t *testing.T) {
	ca := newTestCA(t)
	// Issue a token with a 1-nanosecond TTL — it will be expired by the time we verify.
	ti := identity.NewTokenIssuer(ca.Key(), "https://registry.nexusagentprotocol.com", time.Nanosecond)

	token, err := ti.Issue("agent://nexusagentprotocol.com/a/agent_x", nil)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond)

	_, err = ti.Verify(token)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestTokenIssuer_Verify_tamperedSignature(t *testing.T) {
	ti := newTestTokenIssuer(t)

	token, _ := ti.Issue("agent://nexusagentprotocol.com/a/agent_x", nil)
	// Flip a mid-signature character to corrupt the decoded bytes.
	// The last character must not be flipped: for a 4096-bit RSA key the
	// 512-byte signature encodes to base64url with 4 padding bits in the
	// final character, which decoders discard — so flipping it has no effect.
	parts := strings.Split(token, ".")
	sig := []byte(parts[2])
	mid := len(sig) / 2
	if sig[mid] == 'a' {
		sig[mid] = 'b'
	} else {
		sig[mid] = 'a'
	}
	tampered := parts[0] + "." + parts[1] + "." + string(sig)

	_, err = ti.Verify(tampered)
	if err == nil {
		t.Error("expected error for tampered token, got nil")
	}
}

func TestTokenIssuer_Verify_wrongIssuer(t *testing.T) {
	ca := newTestCA(t)
	ti1 := identity.NewTokenIssuer(ca.Key(), "https://registry-a.nexusagentprotocol.com", time.Hour)
	ti2 := identity.NewTokenIssuer(ca.Key(), "https://registry-b.nexusagentprotocol.com", time.Hour)

	token, _ := ti1.Issue("agent://nexusagentprotocol.com/a/agent_x", nil)
	_, err := ti2.Verify(token)
	if err == nil {
		t.Error("expected error for wrong issuer, got nil")
	}
}

func TestTokenIssuer_PublicKeyPEM(t *testing.T) {
	ti := newTestTokenIssuer(t)
	pem, err := ti.PublicKeyPEM()
	if err != nil {
		t.Fatalf("PublicKeyPEM() error: %v", err)
	}
	if !strings.HasPrefix(pem, "-----BEGIN PUBLIC KEY-----") {
		t.Errorf("unexpected PEM header: %q", pem[:26])
	}
}

func TestHasScope(t *testing.T) {
	ti := newTestTokenIssuer(t)
	token, _ := ti.Issue("agent://nexusagentprotocol.com/a/agent_x", []string{"agent:resolve", "agent:call"})
	claims, _ := ti.Verify(token)

	if !identity.HasScope(claims, "agent:resolve") {
		t.Error("HasScope(agent:resolve) should be true")
	}
	if identity.HasScope(claims, "agent:admin") {
		t.Error("HasScope(agent:admin) should be false")
	}
	if identity.HasScope(nil, "agent:resolve") {
		t.Error("HasScope(nil, ...) should be false")
	}
}

// err is declared at package level to avoid "declared and not used" in tamper test.
var err error
