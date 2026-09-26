package proxy

import (
	"log"
	"net/http"
	"time"
)

// DefaultRedirectAddr and DefaultProxyAddr preserve current behavior.
const (
	DefaultRedirectAddr = ":80"
	DefaultProxyAddr    = ":443"
)

// StartRedirectServer starts an HTTP server that redirects every request to HTTPS. It preserves host and path while forcing a permanent redirect.
func StartRedirectServer(addr string, errCh chan<- error) *http.Server {
	if addr == "" {
		addr = DefaultRedirectAddr
	}
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := hostOnly(r.Host)
			target := "https://" + host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
	}

	go func() {
		log.Printf("HTTP redirect server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	return srv
}
