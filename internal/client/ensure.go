package client

import (
	"github.com/ppablomunoz/noports/internal/pki"
)

// EnsureProxy ensures the local CA and daemon are running, starting the daemon when needed. It is idempotent and safe to call before every control-plane command.
func EnsureProxy() error {
	if err := pki.EnsureCA(); err != nil {
		return err
	}

	running, err := IsDaemonRunning()
	if err != nil {
		return err
	}

	if !running {
		if err := StartDaemon(); err != nil {
			return err
		}
	}

	return nil
}
