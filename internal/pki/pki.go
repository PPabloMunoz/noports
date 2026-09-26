package pki

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ppablomunoz/noports/internal/paths"
)

const (
	// CASubjectName is the CommonName of the local root CA.
	CASubjectName = "Noports Local CA"
	// CACertFile and CAKeyFile are file names inside the certs dir.
	CACertFile = "ca.pem"
	CAKeyFile  = "ca-key.pem"
	// leafRenewBeforeExpiry renews 3-month leafs when under 30 days remain.
	leafRenewBeforeExpiry = 30 * 24 * time.Hour
	// caRenewBeforeExpiry rotates the 10-year CA when under 90 days remain.
	caRenewBeforeExpiry = 90 * 24 * time.Hour
)

// GetCACertPath returns the path to the local CA certificate file. It resolves the certs directory first and joins it with the CA filename.
func GetCACertPath() (string, error) {
	dir, err := paths.CertsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CACertFile), nil
}

// GetCAKeyPath returns the path to the local CA private key file. It resolves the certs directory first and joins it with the key filename.
func GetCAKeyPath() (string, error) {
	dir, err := paths.CertsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CAKeyFile), nil
}

func writeKeyPair(certPath, keyPath string, derBytes []byte, key *ecdsa.PrivateKey) error {
	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", certPath, err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		_ = certOut.Close()
		return fmt.Errorf("failed to write data to %s: %w", certPath, err)
	}
	if err := certOut.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", certPath, err)
	}
	if err := os.Chmod(certPath, 0o644); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", certPath, err)
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", keyPath, err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		_ = keyOut.Close()
		return fmt.Errorf("unable to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		_ = keyOut.Close()
		return fmt.Errorf("failed to write to %s: %w", keyPath, err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", keyPath, err)
	}
	if err := os.Chmod(keyPath, 0o600); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", keyPath, err)
	}
	return nil
}

// notAfter returns the NotAfter expiry of the PEM-encoded certificate at certPath. It reads and parses the file on each call.
func notAfter(certPath string) (time.Time, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return time.Time{}, err
	}
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode %s", certPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}

// IsCertFresh reports whether a loaded TLS cert stays valid beyond the leaf renewal window. Certs without a parsed Leaf are treated as stale so the caller reloads them from disk.
func IsCertFresh(cert *tls.Certificate) bool {
	if cert == nil || cert.Leaf == nil {
		return false
	}
	return time.Now().Add(leafRenewBeforeExpiry).Before(cert.Leaf.NotAfter)
}
