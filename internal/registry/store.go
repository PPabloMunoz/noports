package registry

import (
	"fmt"
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

// Add inserts a new route; it fails if the hostname already exists.
func (s *Store) Add(r Route) error {
	if r.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	if r.Port == 0 {
		return fmt.Errorf("port is required")
	}
	s.mu.Lock()
	if _, ok := s.routes[r.Hostname]; ok {
		s.mu.Unlock()
		return fmt.Errorf("%s hostname already exists. Remove the previous first", r.Hostname)
	}
	s.routes[r.Hostname] = r
	s.mu.Unlock()
	return Save(s)
}

// Get returns the route for hostname.
func (s *Store) Get(hostname string) (*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[hostname]
	if !ok {
		return nil, fmt.Errorf("route with %s hostname is not registered", hostname)
	}
	cp := r
	return &cp, nil
}

// Remove deletes the route for hostname.
func (s *Store) Remove(hostname string) error {
	s.mu.Lock()
	delete(s.routes, hostname)
	s.mu.Unlock()
	return Save(s)
}
