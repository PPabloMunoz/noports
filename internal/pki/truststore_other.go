//go:build !(darwin || linux)

package pki

import "fmt"

func IsCATrusted() (bool, error) {
	return false, fmt.Errorf("OS is not supported yet")
}

func installCA() error {
	return fmt.Errorf("OS is not supported yet")
}

// UninstallCA is a stub on unsupported platforms.
func UninstallCA() error {
	return fmt.Errorf("OS is not supported yet")
}
