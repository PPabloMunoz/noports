package app

import (
	"log/slog"
	"time"

	"github.com/ppablomunoz/noports/internal/proxy"
	"github.com/ppablomunoz/noports/internal/registry"
)

// orphanSweepInterval is how often the daemon reaps `run` routes whose
// wrapper or child died without unregistering.
const orphanSweepInterval = 30 * time.Second

// sweepOrphans prunes run routes whose processes are gone and persists the result. Aliases are never touched; a dead wrapper frees the name even if the orphaned child still listens.
func sweepOrphans(store *registry.Store) {
	pruned, err := store.PruneAndPersist(registry.RouteAlive)
	for _, r := range pruned {
		invalidateCachedCert(r.Hostname)
		proxy.InvalidateHost(r.Hostname)
	}
	if err != nil {
		slog.Warn("failed to persist pruned routes", "err", err)
	}
}

// startOrphanSweep reaps orphaned run routes on every tick until done closes. It runs in its own goroutine and returns when done closes.
func startOrphanSweep(store *registry.Store, done <-chan struct{}) {
	ticker := time.NewTicker(orphanSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sweepOrphans(store)
		case <-done:
			return
		}
	}
}
