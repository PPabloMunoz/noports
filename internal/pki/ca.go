package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ppablomunoz/noports/internal/paths"
)

// EnsureCA generates the local CA if missing and installs it into the OS trust store.
func EnsureCA() error {
	if err := paths.EnsureDirs(); err != nil {
		return err
	}

	certPath, err := GetCACertPath()
	if err != nil {
		return fmt.Errorf("failed to get CA cert path: %w", err)
	}
	keyPath, err := GetCAKeyPath()
	if err != nil {
		return fmt.Errorf("failed to get CA key path: %w", err)
	}

	if caNeedsRotation(certPath, keyPath) {
		fmt.Println("CA certificate missing, expired, or near expiry — rotating")
		if err := generateCA(); err != nil {
			return err
		}
		// Leafs were signed by the old CA; drop them so they are re-issued on demand.
		if err := removeStaleLeafs(); err != nil {
			return err
		}
	}

	trusted, err := IsCATrusted()
	if err != nil {
		return err
	}
	if !trusted {
		if err := installCA(); err != nil {
			return err
		}
	}

	return nil
}

// caNeedsRotation reports whether the CA must be (re)generated. It returns true when files are missing, the cert is corrupt, expired, or inside the renewal window.
func caNeedsRotation(certPath, keyPath string) bool {
	if !paths.Exists(certPath) || !paths.Exists(keyPath) {
		return true
	}
	expiry, err := notAfter(certPath)
	if err != nil {
		return true
	}
	return !time.Now().Add(caRenewBeforeExpiry).Before(expiry)
}

// removeStaleLeafs deletes every *.pem in the certs dir except the CA pair. Called after CA rotation since old leafs are no longer verifiable; they are re-issued on demand.
func removeStaleLeafs() error {
	dir, err := paths.CertsDir()
	if err != nil {
		return fmt.Errorf("failed to get certificates dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to list certificates dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == CACertFile || name == CAKeyFile || filepath.Ext(name) != ".pem" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale leaf %s: %w", name, err)
		}
	}
	return nil
}

// caCache holds the parsed CA pair so leaf issuance doesn't re-read PEM
// files from disk on every call. Reset on rotation (see generateCA).
var (
	caCacheMu   sync.RWMutex
	caCacheCert *x509.Certificate
	caCacheKey  *ecdsa.PrivateKey
)

func generateCA() error {
	certPath, err := GetCACertPath()
	if err != nil {
		return fmt.Errorf("failed to get CA cert path: %w", err)
	}
	keyPath, err := GetCAKeyPath()
	if err != nil {
		return fmt.Errorf("failed to get CA key path: %w", err)
	}

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("failed to create random serial number for CA: %w", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&caKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal CA public key: %w", err)
	}
	skid := sha1.Sum(pubBytes)

	now := time.Now()

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: CASubjectName,
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		SubjectKeyId:          skid[:],
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&caKey.PublicKey,
		caKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	if err := writeKeyPair(certPath, keyPath, derBytes, caKey); err != nil {
		return err
	}

	// New files on disk: drop the cached pair so the next loadCA re-reads.
	caCacheMu.Lock()
	caCacheCert = nil
	caCacheKey = nil
	caCacheMu.Unlock()

	return nil
}

func loadCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	caCacheMu.RLock()
	if caCacheCert != nil && caCacheKey != nil {
		cert, key := caCacheCert, caCacheKey
		caCacheMu.RUnlock()
		return cert, key, nil
	}
	caCacheMu.RUnlock()
	certPath, err := GetCACertPath()
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}

	certDecoded, _ := pem.Decode(certBytes)
	if certDecoded == nil {
		return nil, nil, fmt.Errorf("failed to decode CA cert")
	}

	cert, err := x509.ParseCertificate(certDecoded.Bytes)
	if err != nil {
		return nil, nil, err
	}

	keyPath, err := GetCAKeyPath()
	if err != nil {
		return nil, nil, err
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	keyDecoded, _ := pem.Decode(keyBytes)
	if keyDecoded == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key")
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(keyDecoded.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, ok := keyAny.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("CA key is not an ECDSA private key (got %T)", keyAny)
	}

	caCacheMu.Lock()
	caCacheCert = cert
	caCacheKey = key
	caCacheMu.Unlock()

	return cert, key, nil
}
