package eebus

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enbility/ship-go/cert"
)

// LoadOrCreateIdentity loads the Steuerbox's own TLS certificate/key from
// dataDir, generating and persisting a new one on first run. The SKI derived
// from this certificate is our own identity on the SHIP network; it must
// stay stable across restarts, otherwise every previously paired device
// would see us as a new, untrusted service on every run.
func LoadOrCreateIdentity(dataDir string) (tls.Certificate, error) {
	certPath := filepath.Join(dataDir, "eecheck-cert.pem")
	keyPath := filepath.Join(dataDir, "eecheck-key.pem")

	if fileExists(certPath) && fileExists(keyPath) {
		c, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			return c, nil
		}
		// fall through and regenerate if the stored files are unreadable/corrupt
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return tls.Certificate{}, fmt.Errorf("creating data dir: %w", err)
	}

	c, err := cert.CreateCertificate("EECheck", "EECheck", "DE", "EECheck-Steuerbox")
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("creating certificate: %w", err)
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]})
	if err := os.WriteFile(certPath, certPem, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("writing certificate: %w", err)
	}

	ecKey, ok := c.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return tls.Certificate{}, fmt.Errorf("unexpected private key type %T", c.PrivateKey)
	}
	keyBytes, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshalling private key: %w", err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	if err := os.WriteFile(keyPath, keyPem, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("writing private key: %w", err)
	}

	return c, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
