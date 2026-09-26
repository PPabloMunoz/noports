package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
)

// Load reads routes.json into s, creating the file when missing. It also prunes orphaned run routes left by crashed wrappers and persists the result.
func (s *Store) Load() error {
	routesPath, err := paths.RoutesFile()
	if err != nil {
		return fmt.Errorf("failed to get routes json file path: %w", err)
	}

	data, err := os.ReadFile(routesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Info("routes file missing, creating empty store", "path", routesPath)
			if err := os.WriteFile(routesPath, []byte("[]"), 0o644); err != nil {
				return fmt.Errorf("failed to create routes.json: %w", err)
			}
			data = []byte("[]")
		} else {
			return fmt.Errorf("failed to open routes file: %w", err)
		}
	}

	var routes []Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return fmt.Errorf("failed to unmarshal routes json: %w", err)
	}

	s.importRoutes(routes)

	// Self-heal routes leaked by abnormally-terminated `run` wrappers
	// (kill -9, hangup, crash): their PIDs are dead but nothing ever sent
	// alias_remove. Persist once if anything was pruned.
	if _, err := s.PruneAndPersist(RouteAlive); err != nil {
		return err
	}
	return nil
}

// Save writes all in-memory routes to routes.json atomically. It marshals the current table and replaces the file via a temporary file and rename.
func (s *Store) Save() error {
	routesPath, err := paths.RoutesFile()
	if err != nil {
		return fmt.Errorf("failed to get routes json file path: %w", err)
	}

	all := s.List()
	list := make([]Route, 0, len(all))
	for _, r := range all {
		list = append(list, r)
	}

	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("failed to marshal routes: %w", err)
	}

	tmp := routesPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}
	if err := os.Rename(tmp, routesPath); err != nil {
		return fmt.Errorf("failed to replace routes file: %w", err)
	}
	return nil
}

// PruneAndPersist removes orphaned routes per isAlive, logs each removal, and persists when anything was pruned. It returns the pruned routes with a nil error on success.
func (s *Store) PruneAndPersist(isAlive func(Route) bool) ([]Route, error) {
	pruned := s.Prune(isAlive)
	if len(pruned) == 0 {
		return nil, nil
	}
	for _, r := range pruned {
		slog.Info("pruned orphaned route", "hostname", r.Hostname, "child_pid", r.PID, "wrapper_pid", r.WrapperPID)
	}
	if err := s.Save(); err != nil {
		return pruned, fmt.Errorf("failed to persist pruned routes: %w", err)
	}
	return pruned, nil
}
