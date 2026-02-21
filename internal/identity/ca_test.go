package identity_test

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-protocol/nexus/internal/identity"
)

func TestCAManager_Create(t *testing.T) {
	dir := t.TempDir()
	ca := identity.NewCAManager(dir)

	if err := ca.Create(); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Cert and key must be on disk.
	for _, name := range []string{"ca.crt", "ca.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	if ca.Cert() == nil {
		t.Error("Cert() returned nil after Create()")
	}
	if ca.Key() == nil {
		t.Error("Key() returned nil after Create()")
	}

	// CA cert must be self-signed.
	pool := ca.CertPool()
	opts := x509.VerifyOptions{Roots: pool}
	if _, err := ca.Cert().Verify(opts); err != nil {
		t.Errorf("CA cert does not verify against itself: %v", err)
	}
}

func TestCAManager_LoadOrCreate_idempotent(t *testing.T) {
	dir := t.TempDir()
	ca1 := identity.NewCAManager(dir)
	if err := ca1.LoadOrCreate(); err != nil {
		t.Fatal(err)
	}
	serial1 := ca1.Cert().SerialNumber.String()

	// Second LoadOrCreate on the same dir must load, not create a new CA.
	ca2 := identity.NewCAManager(dir)
	if err := ca2.LoadOrCreate(); err != nil {
		t.Fatal(err)
	}
	serial2 := ca2.Cert().SerialNumber.String()

	if serial1 != serial2 {
		t.Errorf("LoadOrCreate created a new CA on the second call (serial changed: %s → %s)", serial1, serial2)
	}
}

func TestCAManager_CertPEM(t *testing.T) {
	dir := t.TempDir()
	ca := identity.NewCAManager(dir)
	if err := ca.Create(); err != nil {
		t.Fatal(err)
	}

	pem := ca.CertPEM()
	if len(pem) == 0 {
		t.Error("CertPEM() returned empty bytes")
	}
	if string(pem[:27]) != "-----BEGIN CERTIFICATE-----" {
		t.Errorf("CertPEM() does not start with PEM header: %q", string(pem[:27]))
	}
}
