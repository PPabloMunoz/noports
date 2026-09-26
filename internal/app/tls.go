package app

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ppablomunoz/noports/internal/pki"
	"github.com/ppablomunoz/noports/internal/registry"
)

var (
	// tlsCerts caches per-hostname leaf certs to avoid file I/O on every handshake. Values are read-only after parsing and safe for concurrent use.
	tlsCerts sync.Map
	// tlsLocks shards handshake-time cert creation per hostname so concurrent first handshakes share one issuance instead of racing.
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

// invalidateAllCerts drops every cached leaf cert. Used when the CA file
// changes (rotation in another process) since old leafs are no longer
// verifiable; they are re-issued on demand.
func invalidateAllCerts() {
	tlsCerts.Range(func(k, _ any) bool {
		tlsCerts.Delete(k)
		return true
	})
}

// certSelector picks the TLS certificate for each SNI handshake. It serves the preloaded localhost cert directly and issues per-route leafs on demand.
type certSelector struct {
	store      *registry.Store
	serverCert *tls.Certificate
	caModUnix  atomic.Int64
}

// newCertSelector builds a selector and seeds CA rotation tracking from disk. A missing CA file leaves tracking disabled until the first handshake.
func newCertSelector(store *registry.Store, serverCert *tls.Certificate) *certSelector {
	s := &certSelector{store: store, serverCert: serverCert}
	s.caModUnix.Store(-1)
	if caPath, err := pki.GetCACertPath(); err == nil {
		if st, err := os.Stat(caPath); err == nil {
			s.caModUnix.Store(st.ModTime().UnixNano())
		}
	}
	return s
}

// checkCARotation drops cached leafs when another process rotated the CA. It compares the CA file modtime against the last seen value.
func (s *certSelector) checkCARotation() {
	caPath, err := pki.GetCACertPath()
	if err != nil {
		return
	}
	st, err := os.Stat(caPath)
	if err != nil {
		return
	}
	if mt := st.ModTime().UnixNano(); s.caModUnix.CompareAndSwap(s.caModUnix.Load(), mt) {
		// Modtime changed since last handshake: old leafs were signed
		// by the previous CA. Drop them; slow path re-issues.
		invalidateAllCerts()
	}
}

// getCertificate implements tls.Config.GetCertificate for route hostnames. It returns the preloaded cert for localhost and issues or reuses leaf certs for registered routes.
func (s *certSelector) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := strings.ToLower(strings.TrimSpace(hello.ServerName))
	if host == "" {
		return nil, fmt.Errorf("no SNI provided, cannot select certificate")
	}
	if host == "localhost" {
		return s.serverCert, nil
	}

	// Single normalization for the whole handshake.
	key, err := registry.NormalizeHostname(host)
	if err != nil {
		return nil, err
	}

	s.checkCARotation()

	// Single route lookup on the fast path: a missing route also
	// evicts any stale cached cert for the name.
	if _, err := s.store.Get(key); err != nil {
		invalidateCachedCert(key)
		return nil, fmt.Errorf("%s is not registered", host)
	}

	// Fast path: cached + fresh (beyond the 30d renewal window).
	if cert, ok := loadCachedCert(key); ok {
		if pki.IsCertFresh(cert) {
			return cert, nil
		}
		invalidateCachedCert(key)
	}

	// Slow path: serialize creation per hostname, then re-check
	// so concurrent first handshakes share one result.
	mu := certLockFor(key)
	mu.Lock()
	defer func() {
		mu.Unlock()
		tlsLocks.Delete(key)
	}()

	if cert, ok := loadCachedCert(key); ok && pki.IsCertFresh(cert) {
		return cert, nil
	}

	// Revalidate under lock: the route may have been removed while
	// waiting. Second and last lookup on this path.
	route, err := s.store.Get(key)
	if err != nil {
		invalidateCachedCert(key)
		return nil, fmt.Errorf("%s is not registered", host)
	}
	cert, err := pki.GetLeafTLSCertificate(route.Hostname)
	if err != nil {
		return nil, err
	}
	tlsCerts.Store(key, cert)
	return cert, nil
}
