package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
)

// Load reads routes.json into s, creating the file when missing. It also prunes orphaned run routes left by crashed wrappers and persists the result.
func Load(s *Store) error {
	routesPath, err := paths.RoutesFile()
	if err != nil {
		return fmt.Errorf("failed to get routes json file path: %w", err)
	}

	data, err := os.ReadFile(routesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("routes json file does not exist. Creating it...")
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
	if pruned := s.Prune(RouteAlive); len(pruned) > 0 {
		for _, r := range pruned {
			log.Printf("pruned orphaned route %s (child pid %d, wrapper pid %d gone)", r.Hostname, r.PID, r.WrapperPID)
		}
		if err := Save(s); err != nil {
			return fmt.Errorf("failed to persist pruned routes: %w", err)
		}
	}
	return nil
}

// Save writes all in-memory routes to routes.json atomically. It marshals the current table and replaces the file via a temporary file and rename.
func Save(s *Store) error {
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
