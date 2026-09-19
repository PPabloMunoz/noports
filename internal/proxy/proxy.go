package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// LookupPort resolves a hostname to a backend local port.
// Implemented by internal/app using registry.Store; injected to keep proxy decoupled.
type LookupPort func(host string) (port int, ok bool)

// StartProxyServer starts the HTTPS reverse proxy.
// certFile/keyFile are the default leaf cert (for "server"); SNI selection uses getCertificate.
// dashboard serves the "localhost" host; if nil, localhost returns 404.
func StartProxyServer(addr string, lookup LookupPort, getCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error), certFile, keyFile string, dashboard http.Handler, errCh chan<- error) (*http.Server, error) {
	if addr == "" {
		addr = DefaultProxyAddr
	}
	// TODO: websocket support for HMR / hot reload.
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := normalizeHost(r.Host)

			if host == "localhost" {
				if dashboard == nil {
					http.NotFound(w, r)
					return
				}
				dashboard.ServeHTTP(w, r)
				return
			}

			port, ok := lookup(host)
			if !ok {
				http.Error(w, fmt.Sprintf("error getting %s: not registered", host), http.StatusNotFound)
				return
			}
			target, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse backend URL: %v", err), http.StatusServiceUnavailable)
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ServeHTTP(w, r)
		}),
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: getCertificate,
		},
	}

	go func() {
		log.Printf("HTTPS proxy server listening on %s", addr)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	return srv, nil
}

func normalizeHost(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	return strings.ToLower(strings.TrimSpace(host))
}
