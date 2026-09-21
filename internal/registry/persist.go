package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/ppablomunoz/noports/internal/paths"
)

// Load reads routes.json (creating it if missing) into s.
func Load(s *Store) error {
	routesPath, err := paths.GetRoutesFilePath()
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

	// Clean routes
	// Need to delete all routes, and kill processes, with a pid != -1.
	// This should only run when starting daemon so there cannot be any running processes

	validRoutes := []Route{}
	for _, r := range routes {
		if r.PID == -1 {
			validRoutes = append(validRoutes, r)
			continue
		}

		// Kill process if exist
		p, err := os.FindProcess(r.PID)
		if err != nil {
			continue
		}
		_ = p.Signal(os.Kill)
	}

	s.FirstLoad(validRoutes)
	return nil
}

// Save load all routes in store into routes.json file
func Save(s *Store) error {
	routesPath, err := paths.GetRoutesFilePath()
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
