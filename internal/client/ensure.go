package client

import (
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/pki"
)

func EnsureProxy() error {
	// Check CA certificate
	if err := pki.EnsureCA(); err != nil {
		return err
	}

	// Check daemon
	daemonPIDFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return err
	}
	_, err = os.ReadFile(daemonPIDFilePath)
	if err != nil {
		// proxy daemon is not running
		if err := StartProxy(); err != nil {
			return err
		}
	}
	return nil
}
