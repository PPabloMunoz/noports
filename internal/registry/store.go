package registry

import (
	"fmt"
	"log"
	"maps"
	"sync"
)

// Route maps a hostname to a local backend port.
type Route struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	PID      int    `json:"PID"`
}

// Store is a concurrency-safe in-memory route table.
type Store struct {
	mu     sync.RWMutex
	routes map[string]Route
}

// NewStore creates an empty route store.
func NewStore() *Store {
	return &Store{routes: map[string]Route{}}
}

// List returns a copy of all routes.
func (s *Store) List() map[string]Route {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Route, len(s.routes))
	maps.Copy(out, s.routes)
	return out
}

// FirstLoad save all the routes into Store without saving it into routes.json. This function SHOULD
// ONLY be used on the intial load of the daemon process, it overwrite anything saved that has
// the same key
func (s *Store) FirstLoad(routes []Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range routes {
		hostname, err := NormalizeHostname(r.Hostname)
		if err != nil {
			log.Printf("Could not load %v: %v\n", r, err)
			continue
		}
		r.Hostname = hostname
		if r.Port <= 0 {
			log.Printf("port is required. Route: %v\n", r)
			continue
		}
		s.routes[r.Hostname] = r
	}
}

// Add inserts a new route; it fails if the hostname already exists.
// The hostname may be a bare name ("api") or fully-qualified
// ("api.localhost"); it is normalized to lowercase "name.localhost".
func (s *Store) Add(r Route) error {
	hostname, err := NormalizeHostname(r.Hostname)
	if err != nil {
		return err
	}
	r.Hostname = hostname
	if r.Port <= 0 {
		return fmt.Errorf("port is required")
	}
	s.mu.Lock()
	if _, ok := s.routes[r.Hostname]; ok {
		return fmt.Errorf("%s hostname already exists. Remove the previous first", r.Hostname)
	}
	s.routes[r.Hostname] = r
	s.mu.Unlock()
	return Save(s)
}

// Get returns the route for hostname (bare name or "name.localhost", any case).
func (s *Store) Get(hostname string) (*Route, error) {
	normalized, err := NormalizeHostname(hostname)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[normalized]
	if !ok {
		return nil, fmt.Errorf("route with %s hostname is not registered", normalized)
	}
	cp := r
	return &cp, nil
}

// Remove deletes the route for hostname (bare name or "name.localhost", any case).
func (s *Store) Remove(hostname string) error {
	normalized, err := NormalizeHostname(hostname)
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.routes, normalized)
	s.mu.Unlock()
	return Save(s)
}
