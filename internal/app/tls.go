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

// CertCache stores per-hostname leaf certificates to avoid file I/O on every handshake. Values are read-only after parsing and safe for concurrent use. The zero value is ready to use.
type CertCache struct {
	certs sync.Map // key -> *tls.Certificate
	locks sync.Map // key -> *sync.Mutex
	// caModUnix tracks the CA file modtime so a rotation performed by another
	// process drops stale cached leafs instead of serving them. -1 disables tracking.
	caModUnix atomic.Int64
}

// NewCertCache builds an empty cache and seeds CA rotation tracking from disk. A missing CA file leaves tracking disabled until the first handshake.
func NewCertCache() *CertCache {
	c := &CertCache{}
	c.caModUnix.Store(-1)
	if caPath, err := pki.GetCACertPath(); err == nil {
		if st, err := os.Stat(caPath); err == nil {
			c.caModUnix.Store(st.ModTime().UnixNano())
		}
	}
	return c
}

// Get returns the cached cert for key, evicting poisoned entries. A miss or stale entry means the caller must issue or reload the cert.
func (c *CertCache) Get(key string) (*tls.Certificate, bool) {
	v, ok := c.certs.Load(key)
	if !ok {
		return nil, false
	}
	cert, ok := v.(*tls.Certificate)
	if !ok || cert == nil {
		c.certs.Delete(key)
		return nil, false
	}
	return cert, true
}

// Invalidate drops key from the cache. It is called when a route is removed so a re-added name never reuses a stale cert.
func (c *CertCache) Invalidate(key string) {
	c.certs.Delete(key)
}

// InvalidateAll drops every cached leaf cert. It is called when the CA file changes since old leafs are no longer verifiable.
func (c *CertCache) InvalidateAll() {
	c.certs.Range(func(k, _ any) bool {
		c.certs.Delete(k)
		return true
	})
}

// WatchCARotation drops cached leafs when another process rotated the CA. It compares the CA file modtime against the last seen value.
func (c *CertCache) WatchCARotation() {
	caPath, err := pki.GetCACertPath()
	if err != nil {
		return
	}
	st, err := os.Stat(caPath)
	if err != nil {
		return
	}
	if mt := st.ModTime().UnixNano(); c.caModUnix.CompareAndSwap(c.caModUnix.Load(), mt) {
		// Modtime changed since last handshake: old leafs were signed
		// by the previous CA. Drop them; slow path re-issues.
		c.InvalidateAll()
	}
}

// lockFor returns the per-hostname mutex for key, creating it on first use. Callers must delete the entry via unlock when done.
func (c *CertCache) lockFor(key string) *sync.Mutex {
	mu, _ := c.locks.LoadOrStore(key, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// set stores cert for key. It is called after issuance on the slow handshake path.
func (c *CertCache) set(key string, cert *tls.Certificate) {
	c.certs.Store(key, cert)
}

// unlock releases the per-hostname mutex and drops its entry. It runs deferred after the slow handshake path finishes.
func (c *CertCache) unlock(key string, mu *sync.Mutex) {
	mu.Unlock()
	c.locks.Delete(key)
}

// certSelector picks the TLS certificate for each SNI handshake. It serves the preloaded localhost cert directly and issues per-route leafs on demand.
type certSelector struct {
	store      *registry.Store
	serverCert *tls.Certificate
	cache      *CertCache
}

// newCertSelector builds a selector over store backed by cache. The server cert serves localhost without touching the cache.
func newCertSelector(store *registry.Store, serverCert *tls.Certificate, cache *CertCache) *certSelector {
	return &certSelector{store: store, serverCert: serverCert, cache: cache}
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

	s.cache.WatchCARotation()

	// Single route lookup on the fast path: a missing route also
	// evicts any stale cached cert for the name.
	if _, err := s.store.Get(key); err != nil {
		s.cache.Invalidate(key)
		return nil, fmt.Errorf("%s is not registered", host)
	}

	// Fast path: cached + fresh (beyond the 30d renewal window).
	if cert, ok := s.cache.Get(key); ok {
		if pki.IsCertFresh(cert) {
			return cert, nil
		}
		s.cache.Invalidate(key)
	}

	// Slow path: serialize creation per hostname, then re-check
	// so concurrent first handshakes share one result.
	mu := s.cache.lockFor(key)
	mu.Lock()
	defer s.cache.unlock(key, mu)

	if cert, ok := s.cache.Get(key); ok && pki.IsCertFresh(cert) {
		return cert, nil
	}

	// Revalidate under lock: the route may have been removed while
	// waiting. Second and last lookup on this path.
	route, err := s.store.Get(key)
	if err != nil {
		s.cache.Invalidate(key)
		return nil, fmt.Errorf("%s is not registered", host)
	}
	cert, err := pki.GetLeafTLSCertificate(route.Hostname)
	if err != nil {
		return nil, err
	}
	s.cache.set(key, cert)
	return cert, nil
}
