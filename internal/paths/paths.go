package paths

import (
	"os"
	"path/filepath"
)

// DefaultSocketPath is the default unix socket location.
// Override with NOPORTS_SOCKET env var via GetSocketPath.
const DefaultSocketPath = "/tmp/noports.sock"

// GetSocketPath returns the unix socket path, honoring NOPORTS_SOCKET.
func GetSocketPath() string {
	if v := os.Getenv("NOPORTS_SOCKET"); v != "" {
		return v
	}
	return DefaultSocketPath
}

// GetBaseDirPath returns ~/.noports.
func GetBaseDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".noports"), nil
}

// GetCertsDirPath returns ~/.noports/certs.
func GetCertsDirPath() (string, error) {
	base, err := GetBaseDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "certs"), nil
}

// GetPIDFilePath returns ~/.noports/daemon.pid.
func GetPIDFilePath() (string, error) {
	base, err := GetBaseDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon.pid"), nil
}

// GetLogFilePath returns ~/.noports/daemon.log.
func GetLogFilePath() (string, error) {
	base, err := GetBaseDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon.log"), nil
}

// GetErrorLogFilePath returns ~/.noports/daemon-errors.log.
func GetErrorLogFilePath() (string, error) {
	base, err := GetBaseDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "daemon-errors.log"), nil
}

// GetRoutesFilePath returns ~/.noports/routes.json.
func GetRoutesFilePath() (string, error) {
	base, err := GetBaseDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "routes.json"), nil
}
