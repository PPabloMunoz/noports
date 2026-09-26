package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LookupPort resolves a hostname to a backend local port.
// Implemented by internal/app using registry.Store; injected to keep proxy decoupled.
type LookupPort func(host string) (port int, ok bool)

// backendTransport is shared by all cached reverse proxies so backend
// keep-alive connections are reused across requests instead of opening
// (and closing) a new TCP connection per request.
var backendTransport = &http.Transport{
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   20,
	MaxConnsPerHost:       100,
	IdleConnTimeout:       90 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// cachedProxy pairs a built reverse proxy with the backend port it dials.
// If the route's port changes the entry is rebuilt on next hit.
type cachedProxy struct {
	port  int
	proxy *httputil.ReverseProxy
}

// reverseProxies caches one *httputil.ReverseProxy per normalized hostname.
// Values are read-only after Store and safe for concurrent use.
var reverseProxies sync.Map // hostname -> cachedProxy

// Invalidate drops the cached reverse proxy for hostname, if any.
// Called on route remove/prune so a re-added name never reuses a stale port.
func Invalidate(hostname string) {
	key := strings.ToLower(strings.TrimSpace(hostname))
	reverseProxies.Delete(key)
}

// getOrCreateProxy returns the cached proxy for host/port, building and
// caching it on miss or when the backend port changed. Concurrent callers
// may build duplicates; the last Store wins and both are functional.
func getOrCreateProxy(host string, port int) *httputil.ReverseProxy {
	if v, ok := reverseProxies.Load(host); ok {
		if cp, ok := v.(cachedProxy); ok && cp.proxy != nil && cp.port == port {
			return cp.proxy
		}
	}
	target := &url.URL{
		Scheme: "http",
		// localhost, not a bare IP: backends may listen on 127.0.0.1-only
		// (IPv4), ::1-only (IPv6, e.g. `astro dev` / Node), or dual-stack.
		// Go's dialer resolves both and falls back, so hardcoding either
		// address family breaks the other.
		Host: net.JoinHostPort("localhost", strconv.Itoa(port)),
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = backendTransport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("backend for %s on localhost:%d failed: %v", host, port, err)
		http.Error(w, fmt.Sprintf("backend for %s is not reachable on localhost:%d — is it listening on PORT? (%v)", host, port, err), http.StatusBadGateway)
	}
	reverseProxies.Store(host, cachedProxy{port: port, proxy: proxy})
	return proxy
}

// StartProxyServer starts the HTTPS reverse proxy.
// certFile/keyFile are the default leaf cert (for "server"); SNI selection uses getCertificate.
// dashboard serves "localhost" and loopback IPs; if nil, those return 404.
func StartProxyServer(addr string, lookup LookupPort, getCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error), certFile, keyFile string, dashboard http.Handler, errCh chan<- error) (*http.Server, error) {
	if addr == "" {
		addr = DefaultProxyAddr
	}
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := normalizeHost(r.Host)

			if host == "localhost" || isLoopbackHost(host) {
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

			getOrCreateProxy(host, port).ServeHTTP(w, r)
		}),
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			GetCertificate:           getCertificate,
			PreferServerCipherSuites: true,
			CurvePreferences:         []tls.CurveID{tls.X25519, tls.CurveP256},
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

func isLoopbackHost(host string) bool {
	h := strings.Trim(strings.TrimSpace(host), "[]")
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
