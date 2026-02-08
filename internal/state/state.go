package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/koltyakov/ora2csv/internal/storage"
	"github.com/koltyakov/ora2csv/pkg/types"
)

// File manages the state.json file
type File struct {
	mu       sync.RWMutex
	path     string
	entities []types.EntityState
	s3       *storage.S3Client
	s3Key    string // S3 key for state file
}

// Load reads and parses the state file
// If s3 is provided, it will try to load from S3 first, falling back to local file
func Load(path string, s3 *storage.S3Client, s3Key string) (*File, error) {
	var data []byte
	var err error

	// Try S3 first if available
	if s3 != nil && s3Key != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Check if state exists in S3
		exists, err := s3.Exists(ctx, s3Key)
		if err == nil && exists {
			// Download from S3
			data, err = s3.DownloadBytes(ctx, s3Key)
			if err == nil {
				// Successfully downloaded from S3, save local copy
				_ = os.WriteFile(path, data, 0644)
				return parseState(data, path, s3, s3Key)
			}
			// On error, fall through to local file
		}
	}

	// Fall back to local file
	data, err = os.ReadFile(path)
	if err != nil {
		// If local doesn't exist and S3 is enabled, return empty state
		if s3 != nil && os.IsNotExist(err) {
			return &File{
				path:     path,
				entities: []types.EntityState{},
				s3:       s3,
				s3Key:    s3Key,
			}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	return parseState(data, path, s3, s3Key)
}

// parseState parses state data and returns a File
func parseState(data []byte, path string, s3 *storage.S3Client, s3Key string) (*File, error) {
	var entities []types.EntityState
	if err := json.Unmarshal(data, &entities); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &File{
		path:     path,
		entities: entities,
		s3:       s3,
		s3Key:    s3Key,
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

// save writes the state to disk atomically and uploads to S3 if configured
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
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("failed to rename temp file: %w (additionally failed to remove temp file: %v)", err, removeErr)
		}
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Upload to S3 if configured
	if f.s3 != nil && f.s3Key != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := f.s3.UploadBytes(ctx, f.s3Key, data); err != nil {
			return fmt.Errorf("failed to upload state to S3 (key=%s): %w", f.s3Key, err)
		}
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
