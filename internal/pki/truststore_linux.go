//go:build linux

package pki

import "fmt"

func IsCATrusted() (bool, error) {
	return false, fmt.Errorf("linux not supported yet")
}

func installCA() error {
	return fmt.Errorf("linux is not supported yet")
}

// UninstallCA is a stub on unsupported platforms.
func UninstallCA() error {
	return fmt.Errorf("linux is not supported yet")
}
