//go:build darwin

package pki

import (
	"fmt"
	"os"
	"os/exec"
)

func IsCATrusted() (bool, error) {
	cmd := exec.Command("security", "find-certificate", "-c", CASubjectName)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

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

// UninstallCA removes the local CA from the macOS trust store.
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
