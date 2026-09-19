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

	lookup := func(host string) (int, bool) {
		route, err := store.Get(host)
		if err != nil {
			return 0, false
		}
		return route.Port, true
	}
	dashboard := buildDashboardHandler(cfg.DashboardTemplate, store)
	getCertificate := func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		host := strings.ToLower(hello.ServerName)
		if host == "" {
			return nil, fmt.Errorf("no SNI provided, cannot select certificate")
		}
		if host == "localhost" {
			return pki.GetLeafTLSCertificate("server")
		}
		route, err := store.Get(host)
		if err != nil {
			return nil, fmt.Errorf("%s is not registered", host)
		}
		return pki.GetLeafTLSCertificate(route.Hostname)
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
			go ipc.HandleConnection(conn, store)
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
