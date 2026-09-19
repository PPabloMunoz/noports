package pki

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ppablomunoz/noports/internal/paths"
)

const (
	// CASubjectName is the CommonName of the local root CA.
	CASubjectName = "Noports Local CA"
	// CACertFile and CAKeyFile are file names inside the certs dir.
	CACertFile = "ca.pem"
	CAKeyFile  = "ca-key.pem"
)

// GetCACertPath returns ~/.noports/certs/ca.pem.
func GetCACertPath() (string, error) {
	dir, err := paths.GetCertsDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CACertFile), nil
}

// GetCAKeyPath returns ~/.noports/certs/ca-key.pem.
func GetCAKeyPath() (string, error) {
	dir, err := paths.GetCertsDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CAKeyFile), nil
}

func writeCertAndKey(certPath, keyPath string, derBytes []byte, key *rsa.PrivateKey) error {
	certOut, err := os.Create(certPath)
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
	return nil
}
