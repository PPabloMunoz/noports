package app

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/paths"
	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/ppablomunoz/noports/internal/proxy"
	"github.com/ppablomunoz/noports/internal/registry"
)

// Config tunes Run. Zero values select defaults for addresses and disable the dashboard.
type Config struct {
	// RedirectAddr is the HTTP listen address; empty means DefaultRedirectAddr.
	RedirectAddr string
	// ProxyAddr is the HTTPS listen address; empty means DefaultProxyAddr.
	ProxyAddr string
	// DashboardTemplate renders the localhost host; nil means 404.
	DashboardTemplate *template.Template
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

	pidFilePath, err := paths.PIDFile()
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
	if err := store.Load(); err != nil {
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

	selector := newCertSelector(store, serverCert)

	lookup := func(host string) (int, bool) {
		route, err := store.Get(host)
		if err != nil {
			return 0, false
		}
		return route.Port, true
	}
	dashboard := newDashboardHandler(cfg.DashboardTemplate, store)

	httpsServer, err := proxy.StartProxyServer(proxyAddr, lookup, selector.getCertificate, defaultCertPath, defaultKeyPath, dashboard, errCh)
	if err != nil {
		return err
	}

	listener, err := ipc.Listen()
	if err != nil {
		return err
	}
	log.Printf("Socket listening on %s\n", paths.Socket())

	onRemove := func(hostname string) {
		invalidateCachedCert(hostname)
		proxy.InvalidateHost(hostname)
	}

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
			go ipc.HandleConnection(conn, store, onRemove)
		}
	}()

	// Reap `run` routes whose wrapper or child died without unregistering.
	// A run route lives exactly as long as its run session; aliases are never touched.
	sweepDone := make(chan struct{})
	defer close(sweepDone)
	go startOrphanSweep(store, sweepDone)

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
