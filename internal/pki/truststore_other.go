//go:build !(darwin || linux)

package pki

import "fmt"

// IsCATrusted always reports unsupported on platforms without a trust-store backend. It returns an error so callers fail closed instead of silently skipping trust.
func IsCATrusted() (bool, error) {
	return false, fmt.Errorf("OS is not supported yet")
}

// installCA always reports unsupported on platforms without a trust-store backend. It exists so EnsureCA compiles everywhere.
func installCA() error {
	return fmt.Errorf("OS is not supported yet")
}

// UninstallCA is a stub on unsupported platforms. It always returns an error since no trust store integration exists.
func UninstallCA() error {
	return fmt.Errorf("OS is not supported yet")
}
