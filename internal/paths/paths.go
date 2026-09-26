package paths

import (
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
