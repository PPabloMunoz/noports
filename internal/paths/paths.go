package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultSocketPath is the default unix socket location. Override it with NOPORTS_SOCKET via Socket.
const DefaultSocketPath = "/tmp/noports.sock"

// Socket returns the unix socket path, honoring NOPORTS_SOCKET. It never fails and defaults to DefaultSocketPath.
func Socket() string {
	if v := os.Getenv("NOPORTS_SOCKET"); v != "" {
		return v
	}
	return DefaultSocketPath
}

// BaseDir returns ~/.noports. It resolves the user home directory on each call.
func BaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".noports"), nil
}

// CertsDir returns ~/.noports/certs. It resolves BaseDir first and appends the certs segment.
func CertsDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "certs"), nil
}

// PIDFile returns ~/.noports/daemon.pid. It resolves BaseDir first and appends the pid filename.
func PIDFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon.pid"), nil
}

// LogFile returns ~/.noports/daemon.log. It resolves BaseDir first and appends the log filename.
func LogFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon.log"), nil
}

// RoutesFile returns ~/.noports/routes.json. It resolves BaseDir first and appends the routes filename.
func RoutesFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "routes.json"), nil
}

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
