package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"time"
)

const agentKeyBits = 2048

// IssuedCert holds the result of any certificate issuance operation.
type IssuedCert struct {
	CertPEM string
	KeyPEM  string
	Serial  string
	Cert    *x509.Certificate // parsed certificate for in-process use
}

// TLSCertificate converts the PEM-encoded cert+key into a tls.Certificate.
func (ic *IssuedCert) TLSCertificate() (tls.Certificate, error) {
	return tls.X509KeyPair([]byte(ic.CertPEM), []byte(ic.KeyPEM))
}

// Issuer issues and verifies X.509 certificates signed by the Nexus CA.
// In root mode (ca != nil) it uses the CAManager directly.
// In federated mode (intermediateCert/Key != nil) it signs via the intermediate CA
// provided by the NAP root registry.
type Issuer struct {
	ca               *CAManager        // non-nil: root mode
	intermediateCert *x509.Certificate // non-nil: federated mode
	intermediateKey  *rsa.PrivateKey   // non-nil: federated mode
	rootCAPool       *x509.CertPool    // non-nil: federated mode, anchors verification
}

// NewIssuer creates an Issuer backed by the given CAManager (root/standalone mode).
func NewIssuer(ca *CAManager) *Issuer {
	return &Issuer{ca: ca}
}

// NewIssuerWithIntermediate creates an Issuer that signs leaf certificates with
// an intermediate CA issued by the NAP root registry (federated mode).
// rootCAPool is used to verify peer certificates up to the root trust anchor.
func NewIssuerWithIntermediate(
	intermediateCert *x509.Certificate,
	intermediateKey *rsa.PrivateKey,
	rootCAPool *x509.CertPool,
) *Issuer {
	return &Issuer{
		intermediateCert: intermediateCert,
		intermediateKey:  intermediateKey,
		rootCAPool:       rootCAPool,
	}
}

// CACertPEM returns the CA certificate in PEM format.
// In federated mode this returns the intermediate cert PEM so that clients
// can configure their TLS trust against this registry's issuing CA.
func (i *Issuer) CACertPEM() string {
	if i.intermediateCert != nil {
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: i.intermediateCert.Raw}))
	}
	return string(i.ca.CertPEM())
}

// IssueIntermediateCert signs a subordinate CA certificate.
// maxPathLen controls how many further CA levels the intermediate may issue:
//   - 0 = leaf-only (default for most registries)
//   - 1 = can issue one level of sub-intermediates (sub-delegation)
//
// Callable in root mode (CAManager) or intermediate mode when the parent cert
// has MaxPathLen > 0.  The child's MaxPathLen must be strictly less than the
// parent's MaxPathLen when issued by an intermediate.
func (i *Issuer) IssueIntermediateCert(org string, validFor time.Duration, maxPathLen int) (*IssuedCert, error) {
	// Determine signing parent.
	var (
		parentCert *x509.Certificate
		signerKey  interface{}
	)
	switch {
	case i.ca != nil && i.ca.cert != nil && i.ca.key != nil:
		// Root mode — sign with root CA.
		parentCert = i.ca.cert
		signerKey = i.ca.key
	case i.intermediateCert != nil && i.intermediateKey != nil && i.intermediateCert.MaxPathLen > 0:
		// Intermediate mode with sub-delegation authority.
		if maxPathLen >= i.intermediateCert.MaxPathLen {
			return nil, fmt.Errorf("IssueIntermediateCert: child maxPathLen (%d) must be < parent maxPathLen (%d)",
				maxPathLen, i.intermediateCert.MaxPathLen)
		}
		parentCert = i.intermediateCert
		signerKey = i.intermediateKey
	default:
		return nil, fmt.Errorf("IssueIntermediateCert: issuer cannot issue intermediate CAs (no root CA and intermediate MaxPathLen=0)")
	}

	if validFor == 0 {
		validFor = 5 * 365 * 24 * time.Hour
	}

	subKey, err := rsa.GenerateKey(rand.Reader, caKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate intermediate key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("NAP Intermediate CA — %s", org),
			Organization: []string{"Nexus Agent Protocol"},
		},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(validFor),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            maxPathLen,
		MaxPathLenZero:        maxPathLen == 0,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, parentCert, &subKey.PublicKey, signerKey)
	if err != nil {
		return nil, fmt.Errorf("create intermediate certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse intermediate certificate: %w", err)
	}

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(subKey)}))

	return &IssuedCert{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Serial:  serial.Text(16),
		Cert:    cert,
	}, nil
}

// IssueAgentCert issues an X.509 certificate for an agent identity.
//
// ownerCN is the Subject Common Name — for domain-verified agents this is the
// verified domain (e.g. "acme.com"); for NAP-hosted personal agents this is
// the user's chosen display name (e.g. "Jack Merrifield").
//
// ownerEmail, when non-empty, is embedded as an Email SAN and signals a
// NAP-hosted (email-verified) agent. In this mode no DNS SAN is added because
// there is no domain under the owner's control to put there.
// When ownerEmail is empty the certificate behaves as a domain-verified cert:
// the CN is also added as a DNS SAN.
//
// The certificate always contains:
//   - Subject CN: ownerCN
//   - URI SAN:    agentURI  (e.g. agent://nap/finance/taxes/agent_xyz)
//   - DNS SAN:    ownerCN  (domain-verified only, omitted when ownerEmail is set)
//   - Email SAN:  ownerEmail (NAP-hosted only, omitted when empty)
//   - EKU: ClientAuth + ServerAuth
func (i *Issuer) IssueAgentCert(agentURI, ownerCN string, validFor time.Duration, ownerEmail string) (*IssuedCert, error) {
	if err := i.checkSigning(); err != nil {
		return nil, err
	}
	if validFor == 0 {
		validFor = 365 * 24 * time.Hour
	}

	uriSAN, err := url.Parse(agentURI)
	if err != nil {
		return nil, fmt.Errorf("parse agent URI %q: %w", agentURI, err)
	}

	agentKey, err := rsa.GenerateKey(rand.Reader, agentKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate agent key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   ownerCN,
			Organization: []string{"Nexus Agent Protocol"},
		},
		NotBefore:   now.Add(-time.Minute),
		NotAfter:    now.Add(validFor),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		URIs:        []*url.URL{uriSAN},
	}

	if ownerEmail != "" {
		// NAP-hosted personal agent: bind the verified email address.
		template.EmailAddresses = []string{ownerEmail}
	} else {
		// Domain-verified agent: the CN is a DNS hostname the owner controls.
		template.DNSNames = []string{ownerCN}
	}

	return i.sign(template, &agentKey.PublicKey, agentKey)
}

// AgentOwnerEmailFromCert extracts the verified owner email address from a
// NAP-hosted agent certificate's Email SANs. Returns an empty string (no
// error) when the certificate carries no email SAN — this is expected for
// domain-verified agents.
func AgentOwnerEmailFromCert(cert *x509.Certificate) string {
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	return ""
}

// IssueServerCert issues a TLS server certificate for the registry itself.
func (i *Issuer) IssueServerCert(dnsNames []string, ips []net.IP, validFor time.Duration) (*IssuedCert, error) {
	if err := i.checkSigning(); err != nil {
		return nil, err
	}
	if validFor == 0 {
		validFor = 365 * 24 * time.Hour
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, agentKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Nexus Registry",
			Organization: []string{"Nexus Agent Protocol"},
		},
		NotBefore:   now.Add(-time.Minute),
		NotAfter:    now.Add(validFor),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}

	return i.sign(template, &serverKey.PublicKey, serverKey)
}

// VerifyAgentCert parses and verifies a PEM-encoded agent certificate against the CA.
func (i *Issuer) VerifyAgentCert(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	return i.VerifyPeerCert(cert)
}

// VerifyPeerCert verifies a parsed certificate against the CA (used by mTLS middleware).
// In federated mode the verification chain goes up to the root CA pool with the
// intermediate as an intermediary.
func (i *Issuer) VerifyPeerCert(cert *x509.Certificate) (*x509.Certificate, error) {
	var opts x509.VerifyOptions
	if i.rootCAPool != nil && i.intermediateCert != nil {
		// Federated: root pool as Roots, intermediate in Intermediates.
		intermediates := x509.NewCertPool()
		intermediates.AddCert(i.intermediateCert)
		opts = x509.VerifyOptions{
			Roots:         i.rootCAPool,
			Intermediates: intermediates,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
	} else {
		opts = x509.VerifyOptions{
			Roots:     i.ca.CertPool(),
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
	}
	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("certificate not trusted: %w", err)
	}
	return cert, nil
}

// CertPool returns the pool appropriate for this issuer mode.
// In federated mode returns the root CA pool (which anchors the full chain).
func (i *Issuer) CertPool() *x509.CertPool {
	if i.rootCAPool != nil {
		return i.rootCAPool
	}
	return i.ca.CertPool()
}

// AgentURIFromCert extracts the agent:// URI from a certificate's URI SANs.
func AgentURIFromCert(cert *x509.Certificate) (string, error) {
	for _, u := range cert.URIs {
		if u.Scheme == "agent" {
			return u.String(), nil
		}
	}
	return "", fmt.Errorf("no agent:// URI SAN found in certificate (CN=%s)", cert.Subject.CommonName)
}

// checkSigning returns an error if neither root CA nor intermediate is ready.
func (i *Issuer) checkSigning() error {
	if i.intermediateCert != nil && i.intermediateKey != nil {
		return nil
	}
	if i.ca != nil && i.ca.cert != nil && i.ca.key != nil {
		return nil
	}
	return fmt.Errorf("CA not loaded; call LoadOrCreate first or configure intermediate CA")
}

// sign creates and signs a certificate.
// Uses the intermediate CA when in federated mode, otherwise falls back to the root CAManager.
func (i *Issuer) sign(template *x509.Certificate, pub *rsa.PublicKey, priv *rsa.PrivateKey) (*IssuedCert, error) {
	var (
		parent    *x509.Certificate
		signerKey interface{}
	)
	if i.intermediateCert != nil && i.intermediateKey != nil {
		parent = i.intermediateCert
		signerKey = i.intermediateKey
	} else {
		parent = i.ca.cert
		signerKey = i.ca.key
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, signerKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse issued certificate: %w", err)
	}

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))

	return &IssuedCert{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Serial:  template.SerialNumber.Text(16),
		Cert:    cert,
	}, nil
}
