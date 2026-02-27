package identity_test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/jmerrifield20/NexusAgentProtocol/internal/identity"
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

func TestIssuer_IssueAgentCert_domain(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	agentURI := "agent://nexusagentprotocol.com/finance/taxes/agent_testxyz"
	cert, err := issuer.IssueAgentCert(agentURI, "example.com", 24*time.Hour, "")
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

	// Domain-verified cert must have DNS SAN, no email SAN.
	if len(verified.DNSNames) == 0 {
		t.Error("domain-verified cert: expected DNS SAN, got none")
	}
	if email := identity.AgentOwnerEmailFromCert(verified); email != "" {
		t.Errorf("domain-verified cert: expected no email SAN, got %q", email)
	}
}

func TestIssuer_IssueAgentCert_napHosted(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	agentURI := "agent://nap/finance/taxes/agent_abc"
	ownerEmail := "jack@example.com"
	ownerDisplayName := "Jack Merrifield"

	cert, err := issuer.IssueAgentCert(agentURI, ownerDisplayName, 24*time.Hour, ownerEmail)
	if err != nil {
		t.Fatalf("IssueAgentCert() (nap_hosted) error: %v", err)
	}

	verified, err := issuer.VerifyAgentCert(cert.CertPEM)
	if err != nil {
		t.Fatalf("VerifyAgentCert() failed: %v", err)
	}

	// CN must be the user's display name.
	if verified.Subject.CommonName != ownerDisplayName {
		t.Errorf("CN: got %q, want %q", verified.Subject.CommonName, ownerDisplayName)
	}

	// Email SAN must carry the verified email.
	if got := identity.AgentOwnerEmailFromCert(verified); got != ownerEmail {
		t.Errorf("email SAN: got %q, want %q", got, ownerEmail)
	}

	// NAP-hosted cert must have no DNS SAN.
	if len(verified.DNSNames) != 0 {
		t.Errorf("nap_hosted cert: unexpected DNS SANs: %v", verified.DNSNames)
	}

	// URI SAN must still be present.
	extractedURI, err := identity.AgentURIFromCert(verified)
	if err != nil {
		t.Fatalf("AgentURIFromCert() error: %v", err)
	}
	if extractedURI != agentURI {
		t.Errorf("URI SAN: got %q, want %q", extractedURI, agentURI)
	}
}

func TestIssuer_AgentURIFromCert(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	agentURI := "agent://nexusagentprotocol.com/assistant/agent_abc123"
	cert, err := issuer.IssueAgentCert(agentURI, "example.com", time.Hour, "")
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
	cert, err := issuer1.IssueAgentCert("agent://nexusagentprotocol.com/a/agent_x", "example.com", time.Hour, "")
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

	cert, err := issuer.IssueAgentCert("agent://nexusagentprotocol.com/test/agent_peer", "peer.example.com", time.Hour, "")
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

func TestIssueIntermediateCert_MaxPathLenZero(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	issued, err := issuer.IssueIntermediateCert("acme.com", 365*24*time.Hour, 0)
	if err != nil {
		t.Fatalf("IssueIntermediateCert(maxPathLen=0) error: %v", err)
	}

	if !issued.Cert.IsCA {
		t.Error("expected IsCA=true")
	}
	if issued.Cert.MaxPathLen != 0 {
		t.Errorf("MaxPathLen: got %d, want 0", issued.Cert.MaxPathLen)
	}
	if !issued.Cert.MaxPathLenZero {
		t.Error("expected MaxPathLenZero=true for maxPathLen=0")
	}
}

func TestIssueIntermediateCert_MaxPathLenOne(t *testing.T) {
	ca := newTestCA(t)
	issuer := identity.NewIssuer(ca)

	issued, err := issuer.IssueIntermediateCert("gov.kr", 365*24*time.Hour, 1)
	if err != nil {
		t.Fatalf("IssueIntermediateCert(maxPathLen=1) error: %v", err)
	}

	if !issued.Cert.IsCA {
		t.Error("expected IsCA=true")
	}
	if issued.Cert.MaxPathLen != 1 {
		t.Errorf("MaxPathLen: got %d, want 1", issued.Cert.MaxPathLen)
	}
	if issued.Cert.MaxPathLenZero {
		t.Error("expected MaxPathLenZero=false for maxPathLen=1")
	}
}

func TestSubDelegation_IntermediateCanIssue(t *testing.T) {
	// Root issues intermediate with MaxPathLen=1.
	ca := newTestCA(t)
	rootIssuer := identity.NewIssuer(ca)

	intermediate, err := rootIssuer.IssueIntermediateCert("gov.kr", 365*24*time.Hour, 1)
	if err != nil {
		t.Fatalf("issue intermediate: %v", err)
	}

	// Build an intermediate-mode issuer from the issued cert+key.
	intermediateCert, intermediateKey, err := identity.LoadCertAndKey(
		[]byte(intermediate.CertPEM),
		[]byte(intermediate.KeyPEM),
	)
	if err != nil {
		t.Fatalf("load intermediate cert+key: %v", err)
	}

	rootPool := ca.CertPool()
	intIssuer := identity.NewIssuerWithIntermediate(intermediateCert, intermediateKey, rootPool)

	// Intermediate (MaxPathLen=1) issues sub-intermediate with MaxPathLen=0.
	subIntermediate, err := intIssuer.IssueIntermediateCert("molit.go.kr", 365*24*time.Hour, 0)
	if err != nil {
		t.Fatalf("issue sub-intermediate: %v", err)
	}

	if !subIntermediate.Cert.IsCA {
		t.Error("sub-intermediate: expected IsCA=true")
	}
	if subIntermediate.Cert.MaxPathLen != 0 {
		t.Errorf("sub-intermediate MaxPathLen: got %d, want 0", subIntermediate.Cert.MaxPathLen)
	}
	if !subIntermediate.Cert.MaxPathLenZero {
		t.Error("sub-intermediate: expected MaxPathLenZero=true")
	}
}

func TestSubDelegation_CannotExceedParent(t *testing.T) {
	// Root issues intermediate with MaxPathLen=1.
	ca := newTestCA(t)
	rootIssuer := identity.NewIssuer(ca)

	intermediate, err := rootIssuer.IssueIntermediateCert("gov.kr", 365*24*time.Hour, 1)
	if err != nil {
		t.Fatalf("issue intermediate: %v", err)
	}

	intermediateCert, intermediateKey, err := identity.LoadCertAndKey(
		[]byte(intermediate.CertPEM),
		[]byte(intermediate.KeyPEM),
	)
	if err != nil {
		t.Fatalf("load intermediate cert+key: %v", err)
	}

	rootPool := ca.CertPool()
	intIssuer := identity.NewIssuerWithIntermediate(intermediateCert, intermediateKey, rootPool)

	// Trying to issue a sub-intermediate with MaxPathLen=1 (same as parent) must fail.
	_, err = intIssuer.IssueIntermediateCert("molit.go.kr", 365*24*time.Hour, 1)
	if err == nil {
		t.Error("expected error when child maxPathLen >= parent maxPathLen, got nil")
	}

	// MaxPathLen=2 (greater than parent) must also fail.
	_, err = intIssuer.IssueIntermediateCert("molit.go.kr", 365*24*time.Hour, 2)
	if err == nil {
		t.Error("expected error when child maxPathLen > parent maxPathLen, got nil")
	}
}
