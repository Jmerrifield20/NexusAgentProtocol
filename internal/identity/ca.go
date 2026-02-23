package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	caCertFile = "ca.crt"
	caKeyFile  = "ca.key"
	caKeyBits  = 4096
)

// CAManager manages the Nexus Certificate Authority lifecycle.
// It creates and persists a root CA to disk on first run, then reloads it on
// subsequent starts. All agent and server certificates are signed by this CA.
type CAManager struct {
	dir  string
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

// NewCAManager returns a CAManager that stores the CA files in dir.
func NewCAManager(dir string) *CAManager {
	return &CAManager{dir: dir}
}

// LoadOrCreate loads the CA from disk if it exists; creates a new one otherwise.
func (m *CAManager) LoadOrCreate() error {
	if err := m.Load(); err == nil {
		return nil
	}
	return m.Create()
}

// Load reads an existing CA cert and key from the configured directory.
func (m *CAManager) Load() error {
	certPEM, err := os.ReadFile(filepath.Join(m.dir, caCertFile))
	if err != nil {
		return fmt.Errorf("read CA cert: %w", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(m.dir, caKeyFile))
	if err != nil {
		return fmt.Errorf("read CA key: %w", err)
	}
	cert, key, err := decodeCertAndKey(certPEM, keyPEM)
	if err != nil {
		return err
	}
	m.cert = cert
	m.key = key
	return nil
}

// Create generates a new 4096-bit RSA CA, saves it to disk, and activates it.
func (m *CAManager) Create() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("create cert dir %q: %w", m.dir, err)
	}

	key, err := rsa.GenerateKey(rand.Reader, caKeyBits)
	if err != nil {
		return fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Nexus Agent Protocol CA",
			Organization: []string{"Nexus Agent Protocol"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now().UTC().Add(-time.Minute),
		NotAfter:              time.Now().UTC().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	if err := os.WriteFile(filepath.Join(m.dir, caCertFile), certPEM, 0o644); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, caKeyFile), keyPEM, 0o600); err != nil {
		return fmt.Errorf("write CA key: %w", err)
	}

	m.cert = cert
	m.key = key
	return nil
}

// Cert returns the loaded CA certificate.
func (m *CAManager) Cert() *x509.Certificate { return m.cert }

// Key returns the loaded CA private key.
func (m *CAManager) Key() *rsa.PrivateKey { return m.key }

// CertPEM returns the CA certificate encoded as PEM.
func (m *CAManager) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: m.cert.Raw})
}

// CertPool returns an x509.CertPool containing only this CA certificate.
func (m *CAManager) CertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(m.cert)
	return pool
}

// TLSConfig builds a *tls.Config for the registry HTTPS server.
// It requests (but does not mandate at the TLS layer) a client certificate;
// per-route mTLS enforcement is done by the RequireMTLS middleware.
func (m *CAManager) TLSConfig(serverCert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequestClientCert,
		ClientCAs:    m.CertPool(),
		MinVersion:   tls.VersionTLS13,
	}
}

// decodeCertAndKey parses PEM-encoded certificate and RSA private key bytes.
func decodeCertAndKey(certPEM, keyPEM []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate: %w", err)
	}

	block, _ = pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse private key: %w", err)
	}
	return cert, key, nil
}

// LoadCertAndKey parses PEM-encoded cert + RSA key bytes.
// Exposes the internal decodeCertAndKey for use by main.go federated startup.
func LoadCertAndKey(certPEM, keyPEM []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	return decodeCertAndKey(certPEM, keyPEM)
}

// FetchRootCAPool downloads a CA cert PEM from a URL and returns a CertPool.
// Used in federated mode to anchor trust in the NAP root CA.
func FetchRootCAPool(ctx context.Context, caURL string, timeout time.Duration) (*x509.CertPool, error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, caURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build CA fetch request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch root CA from %s: %w", caURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch root CA: unexpected status %d from %s", resp.StatusCode, caURL)
	}
	pemBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("read root CA body: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("no valid certificates found in root CA response from %s", caURL)
	}
	return pool, nil
}

// randomSerial generates a cryptographically random 128-bit certificate serial.
func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	return serial, nil
}
