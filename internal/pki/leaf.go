package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ppablomunoz/noports/internal/paths"
)

// leafLocks serializes leaf creation per hostname so concurrent first uses
// of the same name share one createLeafCertificate instead of racing on
// the Exists check + file writes. No new dependencies.
var leafLocks sync.Map // hostname -> *sync.Mutex

func leafLockFor(hostname string) *sync.Mutex {
	mu, _ := leafLocks.LoadOrStore(hostname, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func getLeafCertPaths(hostname string) (string, string, error) {
	dir, err := paths.GetCertsDirPath()
	if err != nil {
		return "", "", fmt.Errorf("failed to get certificates dir: %w", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%s.pem", hostname)), filepath.Join(dir, fmt.Sprintf("%s-key.pem", hostname)), nil
}

func createLeafCertificate(hostname string) error {
	caCert, caKey, err := loadCA()
	if err != nil {
		return fmt.Errorf("failed to get CA certificate: %w", err)
	}

	leafCertPath, leafKeyPath, err := getLeafCertPaths(hostname)
	if err != nil {
		return err
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate leaf key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("failed to create random serial number: %w", err)
	}

	now := time.Now()

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("Noports %s", hostname),
		},
		NotBefore:   now,
		NotAfter:    now.AddDate(0, 3, 0), // 3 months
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost", hostname},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		caCert,
		&leafKey.PublicKey,
		caKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create leaf certificate: %w", err)
	}

	if err := writeCertAndKey(leafCertPath, leafKeyPath, derBytes, leafKey); err != nil {
		return err
	}

	fmt.Printf("leaf certificate created for %s\n", hostname)
	return nil
}

// GetLeafCertificatePaths ensures a valid leaf cert exists for hostname and returns its paths.
// Expired, near-expiry (<30d left), corrupt, or missing certs are regenerated.
func GetLeafCertificatePaths(hostname string) (string, string, error) {
	leafCertPath, leafKeyPath, err := getLeafCertPaths(hostname)
	if err != nil {
		return "", "", err
	}

	if leafCertValid(leafCertPath, leafKeyPath) {
		return leafCertPath, leafKeyPath, nil
	}

	mu := leafLockFor(hostname)
	mu.Lock()
	defer mu.Unlock()

	// Re-check under lock: a concurrent caller may have created it.
	if leafCertValid(leafCertPath, leafKeyPath) {
		return leafCertPath, leafKeyPath, nil
	}

	if paths.Exists(leafCertPath) {
		fmt.Printf("renewing leaf certificate for %s (expired or near expiry)\n", hostname)
	}
	if err := createLeafCertificate(hostname); err != nil {
		return "", "", err
	}

	return leafCertPath, leafKeyPath, nil
}

// leafCertValid reports whether both files exist and the cert stays valid
// beyond the renewal window. Anything else means regenerate.
func leafCertValid(leafCertPath, leafKeyPath string) bool {
	if !paths.Exists(leafCertPath) || !paths.Exists(leafKeyPath) {
		return false
	}
	notAfter, err := certNotAfter(leafCertPath)
	if err != nil {
		return false
	}
	return time.Now().Add(leafRenewBeforeExpiry).Before(notAfter)
}

// GetLeafTLSCertificate loads (creating if needed) the TLS cert for hostname.
func GetLeafTLSCertificate(hostname string) (*tls.Certificate, error) {
	leafCertPath, leafKeyPath, err := GetLeafCertificatePaths(hostname)
	if err != nil {
		return nil, err
	}

	certBytes, err := os.ReadFile(leafCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", leafCertPath, err)
	}

	certDecoded, _ := pem.Decode(certBytes)
	if certDecoded == nil {
		return nil, fmt.Errorf("failed to decode leaf cert for %s", hostname)
	}

	keyBytes, err := os.ReadFile(leafKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", leafKeyPath, err)
	}

	keyDecoded, _ := pem.Decode(keyBytes)
	if keyDecoded == nil {
		return nil, fmt.Errorf("failed to decode leaf key for %s", hostname)
	}

	tlsCert, err := tls.X509KeyPair(
		pem.EncodeToMemory(certDecoded),
		pem.EncodeToMemory(keyDecoded),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS key pair: %w", err)
	}

	return &tlsCert, nil
}
