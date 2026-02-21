package client

import (
	"fmt"
	"os"
	"path/filepath"
)

// CertBundle holds the PEM-encoded certificate material for an agent identity.
// It is written to disk by 'nap claim' and read back by NewFromCertDir.
type CertBundle struct {
	// CertPEM is the agent's X.509 certificate issued by the Nexus CA.
	CertPEM string

	// PrivateKeyPEM is the agent's RSA private key. Keep this secret.
	PrivateKeyPEM string

	// CAPEM is the Nexus CA certificate used to verify the registry's TLS cert.
	CAPEM string
}

// LoadCertBundle reads cert.pem, key.pem, and ca.pem from dir.
//
//	bundle, err := client.LoadCertBundle(os.ExpandEnv("$HOME/.nap/certs/example.com"))
func LoadCertBundle(dir string) (*CertBundle, error) {
	read := func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}
		return string(b), nil
	}

	cert, err := read("cert.pem")
	if err != nil {
		return nil, err
	}
	key, err := read("key.pem")
	if err != nil {
		return nil, err
	}
	ca, err := read("ca.pem")
	if err != nil {
		return nil, err
	}
	return &CertBundle{CertPEM: cert, PrivateKeyPEM: key, CAPEM: ca}, nil
}

// NewFromCertDir creates an mTLS-authenticated SDK client by loading the
// certificate bundle written by 'nap claim' from dir.
//
// Additional options (e.g. WithCacheTTL) can be appended:
//
//	c, err := client.NewFromCertDir(
//	    "https://registry.nexus.io",
//	    os.ExpandEnv("$HOME/.nap/certs/example.com"),
//	    client.WithCacheTTL(60*time.Second),
//	)
func NewFromCertDir(registryBase, dir string, opts ...Option) (*Client, error) {
	bundle, err := LoadCertBundle(dir)
	if err != nil {
		return nil, fmt.Errorf("load cert bundle from %q: %w", dir, err)
	}
	return New(registryBase, append([]Option{WithMTLS(bundle.CertPEM, bundle.PrivateKeyPEM, bundle.CAPEM)}, opts...)...)
}

// WithCertDir is the functional-option form of NewFromCertDir.
// It loads cert.pem, key.pem, and ca.pem from dir and configures mTLS.
// Use it when you need to combine cert loading with other New() options:
//
//	c, err := client.New(registryURL,
//	    client.WithCertDir(certDir),
//	    client.WithCacheTTL(30*time.Second),
//	)
func WithCertDir(dir string) Option {
	return func(c *Client) error {
		bundle, err := LoadCertBundle(dir)
		if err != nil {
			return fmt.Errorf("load cert bundle from %q: %w", dir, err)
		}
		return WithMTLS(bundle.CertPEM, bundle.PrivateKeyPEM, bundle.CAPEM)(c)
	}
}
