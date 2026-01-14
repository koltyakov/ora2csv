package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/koltyakov/ora2csv/pkg/types"
)

// File manages the state.json file
type File struct {
	mu       sync.RWMutex
	path     string
	entities []types.EntityState
}

// Load reads and parses the state file
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var entities []types.EntityState
	if err := json.Unmarshal(data, &entities); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &File{
		path:     path,
		entities: entities,
	}, nil
}

// GetEntities returns all entities
func (f *File) GetEntities() []types.EntityState {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]types.EntityState, len(f.entities))
	copy(result, f.entities)
	return result
}

// GetActiveEntities returns only active entities
func (f *File) GetActiveEntities() []types.EntityState {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var active []types.EntityState
	for _, e := range f.entities {
		if e.Active {
			active = append(active, e)
		}
	}
	return active
}

// FindEntity finds an entity by name
func (f *File) FindEntity(name string) (*types.EntityState, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for i := range f.entities {
		if f.entities[i].Entity == name {
			return &f.entities[i], true
		}
	}
	return nil, false
}

// UpdateEntityTimestamp updates the lastRunTime for an entity
func (f *File) UpdateEntityTimestamp(entityName string, timestamp string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	found := false
	for i := range f.entities {
		if f.entities[i].Entity == entityName {
			f.entities[i].LastRunTime = timestamp
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("entity not found: %s", entityName)
	}

	return f.save()
}

// save writes the state to disk atomically
func (f *File) save() error {
	// Sort entities by name for consistent output
	sorted := make([]types.EntityState, len(f.entities))
	copy(sorted, f.entities)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Entity < sorted[j].Entity
	})

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file first
	tmpPath := f.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, f.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetSQLPath returns the path to the SQL file for an entity
func (f *File) GetSQLPath(sqlDir, entityName string) string {
	return filepath.Join(sqlDir, entityName+".sql")
}

// ValidateSQLFiles checks if SQL files exist for all active entities
func (f *File) ValidateSQLFiles(sqlDir string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var missing []string
	for _, e := range f.entities {
		if e.Active {
			sqlPath := f.GetSQLPath(sqlDir, e.Entity)
			if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
				missing = append(missing, e.Entity)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing SQL files for entities: %s", strings.Join(missing, ", "))
	}

	return nil
}

// TotalCount returns the total number of entities
func (f *File) TotalCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.entities)
}

// ActiveCount returns the number of active entities
func (f *File) ActiveCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	count := 0
	for _, e := range f.entities {
		if e.Active {
			count++
		}
	}
	return count
}
