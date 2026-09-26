package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/ppablomunoz/noports/internal/paths"
)

// leafLocks serializes leaf creation per hostname so concurrent first uses of the same name share one issuance instead of racing on the Exists check and file writes.
var leafLocks sync.Map // hostname -> *sync.Mutex

func leafLockFor(hostname string) *sync.Mutex {
	mu, _ := leafLocks.LoadOrStore(hostname, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func leafPaths(hostname string) (string, string, error) {
	dir, err := paths.CertsDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get certificates dir: %w", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%s.pem", hostname)), filepath.Join(dir, fmt.Sprintf("%s-key.pem", hostname)), nil
}

func issueLeaf(hostname string) error {
	caCert, caKey, err := loadCA()
	if err != nil {
		return fmt.Errorf("failed to get CA certificate: %w", err)
	}

	leafCertPath, leafKeyPath, err := leafPaths(hostname)
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

	if err := writeKeyPair(leafCertPath, leafKeyPath, derBytes, leafKey); err != nil {
		return err
	}

	fmt.Printf("leaf certificate created for %s\n", hostname)
	return nil
}

// GetLeafCertificatePaths ensures a valid leaf cert exists for hostname and returns its paths. Expired, near-expiry, corrupt, or missing certs are regenerated.
func GetLeafCertificatePaths(hostname string) (string, string, error) {
	leafCertPath, leafKeyPath, err := leafPaths(hostname)
	if err != nil {
		return "", "", err
	}

	if leafValid(leafCertPath, leafKeyPath) {
		return leafCertPath, leafKeyPath, nil
	}

	mu := leafLockFor(hostname)
	mu.Lock()
	defer func() {
		mu.Unlock()
		leafLocks.Delete(hostname)
	}()

	// Re-check under lock: a concurrent caller may have created it.
	if leafValid(leafCertPath, leafKeyPath) {
		return leafCertPath, leafKeyPath, nil
	}

	if paths.Exists(leafCertPath) {
		fmt.Printf("renewing leaf certificate for %s (expired or near expiry)\n", hostname)
	}
	if err := issueLeaf(hostname); err != nil {
		return "", "", err
	}

	return leafCertPath, leafKeyPath, nil
}

// leafValid reports whether both files exist and the cert stays valid beyond the renewal window. Anything else means regenerate.
func leafValid(leafCertPath, leafKeyPath string) bool {
	if !paths.Exists(leafCertPath) || !paths.Exists(leafKeyPath) {
		return false
	}
	expiry, err := notAfter(leafCertPath)
	if err != nil {
		return false
	}
	return time.Now().Add(leafRenewBeforeExpiry).Before(expiry)
}

// GetLeafTLSCertificate loads the TLS cert for hostname, creating it first when missing. It parses the leaf so freshness checks work without re-reading disk.
func GetLeafTLSCertificate(hostname string) (*tls.Certificate, error) {
	leafCertPath, leafKeyPath, err := GetLeafCertificatePaths(hostname)
	if err != nil {
		return nil, err
	}

	tlsCert, err := tls.LoadX509KeyPair(leafCertPath, leafKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair for %s: %w", hostname, err)
	}
	if len(tlsCert.Certificate) > 0 {
		if leaf, err := x509.ParseCertificate(tlsCert.Certificate[0]); err == nil {
			tlsCert.Leaf = leaf
		}
	}

	return &tlsCert, nil
}
