package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
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

	if !paths.Exists(certPath) || !paths.Exists(keyPath) {
		fmt.Println("Generating CA...")
		if err := generateCA(); err != nil {
			return err
		}
	}

	trusted, err := IsCATrusted()
	if err != nil {
		return err
	}
	if !trusted {
		fmt.Println("CA certificate not found. Installing CA...")
		if err := installCA(); err != nil {
			return err
		}
	}

	return nil
}

func generateCA() error {
	certPath, err := GetCACertPath()
	if err != nil {
		return fmt.Errorf("failed to get CA cert path: %w", err)
	}
	keyPath, err := GetCAKeyPath()
	if err != nil {
		return fmt.Errorf("failed to get CA key path: %w", err)
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("failed to create random serial number for CA: %w", err)
	}

	pubBytes := x509.MarshalPKCS1PublicKey(&caKey.PublicKey)
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

	if err := writeCertAndKey(certPath, keyPath, derBytes, caKey); err != nil {
		return err
	}

	fmt.Println("CA created")
	return nil
}

func loadCA() (*x509.Certificate, *rsa.PrivateKey, error) {
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
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("CA key is not an RSA private key")
	}

	return cert, key, nil
}
