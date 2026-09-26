package paths

import (
	"errors"
	"fmt"
	"os"
)

// Exists reports whether filePath exists. It returns false for missing files and ignores other stat errors.
func Exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}

// EnsureDirs creates ~/.noports/certs including parents. PID-file handling lives in internal/app, not here.
func EnsureDirs() error {
	certsDir, err := CertsDir()
	if err != nil {
		return fmt.Errorf("failed to get certs dir: %w", err)
	}
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create certs dir '%s': %w", certsDir, err)
	}
	return nil
}

// OpenLogFile opens ~/.noports/daemon.log for appending, creating it when missing. Callers must close the returned file.
func OpenLogFile() (*os.File, error) {
	p, err := LogFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get daemon log file path: %w", err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open daemon log file %s: %w", p, err)
	}
	return f, nil
}
