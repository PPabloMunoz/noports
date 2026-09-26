package app

import (
	"log/slog"
	"os"
	"syscall"

	"github.com/ppablomunoz/noports/internal/registry"
)

// notifyWrappers SIGTERMs live run-wrapper processes so their supervised children stop with the daemon. It runs at the top of shutdown while the control socket still accepts, giving wrappers a window to unregister. Aliases are never signalled.
func notifyWrappers(store *registry.Store) {
	for _, r := range store.List() {
		if r.PID == -1 || r.WrapperPID <= 0 {
			continue
		}
		if !registry.SameProcess(r.WrapperPID, r.WrapperStartTime) {
			continue
		}
		p, err := os.FindProcess(r.WrapperPID)
		if err != nil {
			continue
		}
		if err := p.Signal(syscall.SIGTERM); err != nil {
			continue
		}
		slog.Info("signalled run wrapper to stop", "hostname", r.Hostname, "wrapper_pid", r.WrapperPID)
	}
}
