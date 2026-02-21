package identity_test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/internal/identity"
)

// newTestCA is a helper that creates a fresh CA in a temp dir.
func newTestCA(t *testing.T) *identity.CAManager {
	t.Helper()
	ca := identity.NewCAManager(t.TempDir())
	if err := ca.Create(); err != nil {
		t.Fatalf("create test CA: %v", err)
	}
	return ca
}

func TestIssuer_IssueAgentCert(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	agentURI := "agent://nexus.io/finance/taxes/agent_testxyz"
	cert, err := issuer.IssueAgentCert(agentURI, "example.com", 24*time.Hour)
	if err != nil {
		t.Fatalf("IssueAgentCert() error: %v", err)
	}

	if cert.CertPEM == "" {
		t.Error("CertPEM is empty")
	}
	if cert.KeyPEM == "" {
		t.Error("KeyPEM is empty")
	}
	if cert.Serial == "" {
		t.Error("Serial is empty")
	}
	if cert.Cert == nil {
		t.Error("Cert is nil")
	}

	// The cert must verify against our CA.
	verified, err := issuer.VerifyAgentCert(cert.CertPEM)
	if err != nil {
		t.Errorf("VerifyAgentCert() failed: %v", err)
	}
	if verified.Subject.CommonName != "example.com" {
		t.Errorf("CN: got %q, want %q", verified.Subject.CommonName, "example.com")
	}

	// URI SAN must contain the agent URI.
	foundURI := false
	for _, u := range verified.URIs {
		if u.String() == agentURI {
			foundURI = true
			break
		}
	}
	if !foundURI {
		t.Errorf("URI SAN %q not found in certificate", agentURI)
	}
}

func TestIssuer_AgentURIFromCert(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	agentURI := "agent://nexus.io/assistant/agent_abc123"
	cert, err := issuer.IssueAgentCert(agentURI, "example.com", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	extracted, err := identity.AgentURIFromCert(cert.Cert)
	if err != nil {
		t.Fatalf("AgentURIFromCert() error: %v", err)
	}
	if extracted != agentURI {
		t.Errorf("extracted URI: got %q, want %q", extracted, agentURI)
	}
}

func TestIssuer_IssueServerCert(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	cert, err := issuer.IssueServerCert(
		[]string{"localhost"},
		[]net.IP{net.ParseIP("127.0.0.1")},
		365*24*time.Hour,
	)
	if err != nil {
		t.Fatalf("IssueServerCert() error: %v", err)
	}

	// Must be convertible to tls.Certificate.
	tlsCert, err := cert.TLSCertificate()
	if err != nil {
		t.Fatalf("TLSCertificate() error: %v", err)
	}
	if len(tlsCert.Certificate) == 0 {
		t.Error("tls.Certificate has no certificate bytes")
	}
	_ = tls.Certificate{} // ensure import is used
}

func TestIssuer_VerifyAgentCert_rejectsUnknownCA(t *testing.T) {
	ca1 := newTestCA(t)
	ca2 := newTestCA(t)
	issuer1 := identity.NewIssuer(ca1)
	issuer2 := identity.NewIssuer(ca2)

	// Issue with CA1
	cert, err := issuer1.IssueAgentCert("agent://nexus.io/a/agent_x", "example.com", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Verify with CA2 — must fail
	_, err = issuer2.VerifyAgentCert(cert.CertPEM)
	if err == nil {
		t.Error("expected verification to fail with a different CA, but it succeeded")
	}
}

func TestIssuer_VerifyPeerCert(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	cert, err := issuer.IssueAgentCert("agent://nexus.io/test/agent_peer", "peer.example.com", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	verified, err := issuer.VerifyPeerCert(cert.Cert)
	if err != nil {
		t.Errorf("VerifyPeerCert() failed: %v", err)
	}
	if verified.SerialNumber.Cmp(cert.Cert.SerialNumber) != 0 {
		t.Error("returned certificate serial does not match")
	}
}
