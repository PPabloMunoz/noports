//go:build darwin

package pki

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// IsCATrusted reports whether the local CA is installed in the macOS trust store. It returns false without an error when the CA file is missing or no anchor matches it.
func IsCATrusted() (bool, error) {
	certPath, err := GetCACertPath()
	if err != nil {
		return false, err
	}
	caBytes, err := os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	cmd := exec.Command("security", "find-certificate", "-c", CASubjectName, "-p")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return bytes.Contains(out, bytes.TrimSpace(caBytes)), nil
}

// installCA adds the local CA to the macOS trust store. It prompts for sudo since the system keychain is root-owned.
func installCA() error {
	certPath, err := GetCACertPath()
	if err != nil {
		return fmt.Errorf("failed to get CA cert path: %w", err)
	}

	cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", certPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install CA cert: %w", err)
	}
	return nil
}

// UninstallCA removes the local CA from the macOS trust store. It prompts for sudo since the system keychain is root-owned.
func UninstallCA() error {
	cmd := exec.Command("sudo", "security", "delete-certificate", "-t", "-c", CASubjectName)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall CA certificate: %w", err)
	}
	return nil
}
