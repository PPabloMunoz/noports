package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

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

	for _, r := range routes {
		if err := s.Add(r); err != nil {
			log.Printf("%v", err)
		}
	}

	return nil
}

var saveMu sync.Mutex

// Save persists all routes in s to routes.json atomically.
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

	saveMu.Lock()
	defer saveMu.Unlock()

	tmp := routesPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("failed to write routes file: %w", err)
	}
	if err := os.Rename(tmp, routesPath); err != nil {
		return fmt.Errorf("failed to replace routes file: %w", err)
	}
	return nil
}
