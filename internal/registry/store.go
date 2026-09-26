package registry

import (
	"fmt"
	"log"
	"maps"
	"sync"
)

// Route maps a hostname to a local backend port.
//
// PID is the backend child process for `run` routes, or -1 for user-managed
// `alias` routes (no process to watch). WrapperPID is the `run` wrapper
// process that owns the route's lifecycle (-1 for aliases, 0 when recorded
// before wrapper PIDs existed). ChildStartTime/WrapperStartTime are the
// wall-clock start times (Unix ms, see ProcessStartTime) used to detect PID
// reuse; 0 means unknown.
type Route struct {
	Hostname         string `json:"hostname"`
	Port             int    `json:"port"`
	PID              int    `json:"PID"`
	WrapperPID       int    `json:"wrapper_pid,omitempty"`
	ChildStartTime   int64  `json:"child_start_time,omitempty"`
	WrapperStartTime int64  `json:"wrapper_start_time,omitempty"`
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

// importRoutes loads routes into memory without persisting them. It is only used at daemon startup to hydrate the store, overwriting any entry with the same hostname.
func (s *Store) importRoutes(routes []Route) {
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

// Prune removes orphaned run routes whose processes are gone per isAlive. Aliases are never pruned, probing runs lock-free on a snapshot, and deletion is compare-and-delete so re-added routes survive.
func (s *Store) Prune(isAlive func(Route) bool) []Route {
	snapshot := s.List()
	var dead []Route
	for _, r := range snapshot {
		if r.PID == -1 {
			continue
		}
		if isAlive(r) {
			continue
		}
		dead = append(dead, r)
	}
	if len(dead) == 0 {
		return nil
	}
	s.mu.Lock()
	var removed []Route
	for _, r := range dead {
		cur, ok := s.routes[r.Hostname]
		if !ok {
			continue
		}
		if cur.PID != r.PID || cur.WrapperPID != r.WrapperPID ||
			cur.ChildStartTime != r.ChildStartTime || cur.WrapperStartTime != r.WrapperStartTime {
			continue
		}
		delete(s.routes, r.Hostname)
		removed = append(removed, r)
	}
	s.mu.Unlock()
	return removed
}
