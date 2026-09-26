package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/ppablomunoz/noports/internal/proxy"
	"github.com/ppablomunoz/noports/internal/registry"
)

// Config tunes Run. Zero values select defaults.
// DashboardTemplate renders the "localhost" host; nil means 404.
type Config struct {
	RedirectAddr      string
	ProxyAddr         string
	DashboardTemplate *template.Template
}

// orphanSweepInterval is how often the daemon reaps `run` routes whose
// wrapper or child died without unregistering.
const orphanSweepInterval = 30 * time.Second

var (
	// tlsCerts caches per-hostname leaf certs to avoid file I/O + PEM
	// parse on every handshake. Keys are normalized hostnames
	// (see registry.NormalizeHostname). Values are *tls.Certificate,
	// which is read-only after parsing and safe for concurrent use.
	tlsCerts sync.Map
	// tlsLocks shards handshake-time cert creation per hostname so
	// concurrent first handshakes for the same name share one
	// GetLeafTLSCertificate call instead of racing in
	// createLeafCertificate. One entry per hostname; bounded.
	tlsLocks sync.Map
)

// certLockFor returns the per-hostname mutex for key, creating it on first use.
func certLockFor(key string) *sync.Mutex {
	mu, _ := tlsLocks.LoadOrStore(key, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// loadCachedCert returns the cached cert for key, evicting poisoned entries.
func loadCachedCert(key string) (*tls.Certificate, bool) {
	v, ok := tlsCerts.Load(key)
	if !ok {
		return nil, false
	}
	cert, ok := v.(*tls.Certificate)
	if !ok || cert == nil {
		tlsCerts.Delete(key)
		return nil, false
	}
	return cert, true
}

// invalidateCachedCert drops key from the cache.
func invalidateCachedCert(key string) {
	tlsCerts.Delete(key)
}

// Run starts the daemon: dirs/PID, CA, routes, HTTP(S) servers and IPC socket.
// It blocks until ctx is cancelled, a shutdown signal arrives, or a server fails.
func Run(ctx context.Context, cfg Config) error {
	// fd 3 is the readiness pipe *if* we were launched by the parent
	// via cmd.ExtraFiles. If run directly from a terminal, fd 3 won't
	// be open, and the Write below will simply fail — which we ignore.
	readyPipe := os.NewFile(3, "ready")
	if readyPipe != nil {
		defer func() { _ = readyPipe.Close() }()
	}

	if err := paths.EnsureDirs(); err != nil {
		return err
	}

	pidFilePath, err := paths.GetPIDFilePath()
	if err != nil {
		return fmt.Errorf("failed to get daemon pid file path: %w", err)
	}
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return fmt.Errorf("failed to write daemon pid file: %w", err)
	}

	logFile, err := paths.OpenLogFile()
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	log.SetOutput(logFile)

	installed, err := pki.IsCATrusted()
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("CA is not installed")
	}

	store := registry.NewStore()
	if err := registry.Load(store); err != nil {
		return err
	}

	errCh := make(chan error, 2)

	redirectAddr := cfg.RedirectAddr
	if redirectAddr == "" {
		redirectAddr = proxy.DefaultRedirectAddr
	}
	proxyAddr := cfg.ProxyAddr
	if proxyAddr == "" {
		proxyAddr = proxy.DefaultProxyAddr
	}

	httpServer := proxy.StartRedirectServer(redirectAddr, errCh)

	defaultCertPath, defaultKeyPath, err := pki.GetLeafCertificatePaths("server")
	if err != nil {
		return err
	}

	// Preload once so the localhost handshake path does no file I/O.
	serverCert, err := pki.GetLeafTLSCertificate("server")
	if err != nil {
		return err
	}

	lookup := func(host string) (int, bool) {
		route, err := store.Get(host)
		if err != nil {
			return 0, false
		}
		return route.Port, true
	}
	dashboard := buildDashboardHandler(cfg.DashboardTemplate, store)
	getCertificate := func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		host := strings.ToLower(strings.TrimSpace(hello.ServerName))
		if host == "" {
			return nil, fmt.Errorf("no SNI provided, cannot select certificate")
		}
		if host == "localhost" {
			return serverCert, nil
		}

		key, err := registry.NormalizeHostname(host)
		if err != nil {
			return nil, err
		}

		// Fast path: serve from cache, but verify the route still
		// exists so removed/pruned names stop serving a stale cert.
		if cert, ok := loadCachedCert(key); ok {
			if _, err := store.Get(key); err != nil {
				invalidateCachedCert(key)
			} else {
				return cert, nil
			}
		}

		// Slow path: serialize creation per hostname, then re-check
		// so concurrent first handshakes share one result.
		mu := certLockFor(key)
		mu.Lock()
		defer mu.Unlock()

		if cert, ok := loadCachedCert(key); ok {
			if _, err := store.Get(key); err != nil {
				invalidateCachedCert(key)
			} else {
				return cert, nil
			}
		}

		route, err := store.Get(key)
		if err != nil {
			return nil, fmt.Errorf("%s is not registered", host)
		}
		cert, err := pki.GetLeafTLSCertificate(route.Hostname)
		if err != nil {
			return nil, err
		}
		tlsCerts.Store(key, cert)
		return cert, nil
	}

	httpsServer, err := proxy.StartProxyServer(proxyAddr, lookup, getCertificate, defaultCertPath, defaultKeyPath, dashboard, errCh)
	if err != nil {
		return err
	}

	listener, err := ipc.Listen()
	if err != nil {
		return err
	}
	log.Printf("Socket listening on %s\n", paths.GetSocketPath())

	if readyPipe != nil {
		if _, err := readyPipe.Write([]byte{1}); err != nil {
			log.Println("no readiness pipe (likely a manual/foreground run)")
		}
		_ = readyPipe.Close()
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("failed to accept connection to socket: %v", err)
				continue
			}
			go ipc.HandleConnection(conn, store, invalidateCachedCert)
		}
	}()

	// Periodically reap `run` routes whose wrapper or child died without
	// unregistering (kill -9, hangup, crash). Strict rule: a run route lives
	// exactly as long as its run session, so a dead wrapper frees the name
	// even if the orphaned child still listens. Aliases (PID -1) are never
	// touched.
	sweepDone := make(chan struct{})
	defer close(sweepDone)
	go func() {
		ticker := time.NewTicker(orphanSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pruned := store.Prune(registry.RouteAlive)
				if len(pruned) == 0 {
					continue
				}
				for _, r := range pruned {
					invalidateCachedCert(r.Hostname)
					log.Printf("pruned orphaned route %s (child pid %d, wrapper pid %d gone)", r.Hostname, r.PID, r.WrapperPID)
				}
				if err := registry.Save(store); err != nil {
					log.Printf("failed to persist pruned routes: %v", err)
				}
			case <-sweepDone:
				return
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-errCh:
		log.Printf("server error: %v\n", err)
	case sig := <-stop:
		log.Printf("received signal: %v, shutting down\n", sig)
	case <-ctx.Done():
		log.Printf("context cancelled, shutting down: %v", ctx.Err())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown http server: %w", err)
	}
	if err := httpsServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown https server: %w", err)
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("failed to close socket listener: %w", err)
	}
	if err := os.Remove(pidFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", pidFilePath, err)
	}

	log.Println("SHUTDOWN COMPLETE")
	return nil
}

// dashboardData is the template input for web/index.html.tmpl.
type dashboardData struct {
	Routes []registry.Route
}

// buildDashboardHandler returns an http.Handler that renders tmpl with the
// active routes on every request. Nil tmpl means nil handler (proxy 404s localhost).
func buildDashboardHandler(tmpl *template.Template, store *registry.Store) http.Handler {
	if tmpl == nil {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		all := store.List()
		routes := make([]registry.Route, 0, len(all))
		for _, route := range all {
			routes = append(routes, route)
		}
		sort.Slice(routes, func(i, j int) bool { return routes[i].Hostname < routes[j].Hostname })
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, dashboardData{Routes: routes}); err != nil {
			http.Error(w, "failed to render dashboard", http.StatusInternalServerError)
		}
	})
}
