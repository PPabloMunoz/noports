package client

import (
	"github.com/ppablomunoz/noports/internal/pki"
)

func EnsureProxy() error {
	if err := pki.EnsureCA(); err != nil {
		return err
	}

	running, err := IsDaemonRunning()
	if err != nil {
		return err
	}

	if !running {
		if err := StartProxy(); err != nil {
			return err
		}
	}

	return nil
}
