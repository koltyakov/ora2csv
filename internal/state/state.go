package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/koltyakov/ora2csv/pkg/types"
)

type remoteStore interface {
	DownloadBytesVersion(ctx context.Context, key string) ([]byte, *string, error)
	UploadBytesCAS(ctx context.Context, key string, data []byte, expectedETag *string) (string, error)
}

// File manages the state.json file
type File struct {
	mu       sync.RWMutex
	path     string
	entities []types.EntityState
	s3       remoteStore
	s3Key    string // S3 key for state file
	s3ETag   *string
}

// Load reads and parses the state file
// If s3 is provided, it will try to load from S3 first, falling back to local file
func Load(path string, s3 remoteStore, s3Key string) (*File, error) {
	var data []byte
	var err error

	// Try S3 first if available
	s3StateMissing := false
	if s3 != nil && s3Key != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, etag, err := s3.DownloadBytesVersion(ctx, s3Key)
		if err != nil {
			return nil, fmt.Errorf("failed to download S3 state file %s: %w", s3Key, err)
		}
		if etag != nil {
			// Validate remote state before replacing the local recovery copy.
			parsed, err := parseState(data, path, s3, s3Key)
			if err != nil {
				return nil, err
			}
			if err := replaceFileAtomic(path, data); err != nil {
				return nil, fmt.Errorf("failed to write local state file %s: %w", path, err)
			}
			parsed.s3ETag = etag
			return parsed, nil
		}
		s3StateMissing = true
	}

	// Fall back to local file
	data, err = os.ReadFile(path)
	if err != nil {
		// If neither local nor S3 state exists, start with an empty state.
		if s3 != nil && s3StateMissing && os.IsNotExist(err) {
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
func parseState(data []byte, path string, s3 remoteStore, s3Key string) (*File, error) {
	type rawEntityState struct {
		Entity      *string `json:"entity"`
		LastRunTime *string `json:"lastRunTime"`
		Active      *bool   `json:"active"`
	}

	var rawEntities []rawEntityState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawEntities); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}
	if rawEntities == nil {
		return nil, fmt.Errorf("failed to parse state file: expected a JSON array")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	entities := make([]types.EntityState, len(rawEntities))
	seen := make(map[string]struct{}, len(rawEntities))
	for i, raw := range rawEntities {
		if raw.Entity == nil || raw.LastRunTime == nil || raw.Active == nil {
			return nil, fmt.Errorf("failed to parse state file: entity at index %d must define entity, lastRunTime, and active", i)
		}
		if err := validateEntityName(*raw.Entity); err != nil {
			return nil, fmt.Errorf("failed to parse state file: entity at index %d: %w", i, err)
		}
		if _, exists := seen[*raw.Entity]; exists {
			return nil, fmt.Errorf("failed to parse state file: duplicate entity %q", *raw.Entity)
		}
		seen[*raw.Entity] = struct{}{}

		entity := types.EntityState{
			Entity:      *raw.Entity,
			LastRunTime: *raw.LastRunTime,
			Active:      *raw.Active,
		}
		if _, err := entity.GetLastRunTime(); err != nil {
			return nil, fmt.Errorf("failed to parse state file: entity %q has invalid lastRunTime: %w", entity.Entity, err)
		}
		entities[i] = entity
	}

	return &File{
		path:     path,
		entities: entities,
		s3:       s3,
		s3Key:    s3Key,
	}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected data after state array")
		}
		return err
	}
	return nil
}

func validateEntityName(name string) error {
	if name == "" {
		return fmt.Errorf("entity name is required")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("entity name %q must not contain surrounding whitespace", name)
	}
	if name == "." || name == ".." || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return fmt.Errorf("invalid entity name %q", name)
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("entity name %q must be a single path component", name)
	}
	return nil
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
			entity := f.entities[i]
			return &entity, true
		}
	}
	return nil, false
}

// UpdateEntityTimestamp updates the lastRunTime for an entity
func (f *File) UpdateEntityTimestamp(entityName string, timestamp string) error {
	parsed, err := time.ParseInLocation("2006-01-02T15:04:05", timestamp, time.UTC)
	if err != nil {
		return fmt.Errorf("invalid timestamp for entity %s: %w", entityName, err)
	}
	_, err = f.AdvanceEntityTimestamp(entityName, parsed)
	return err
}

// AdvanceEntityTimestamp persists a timestamp only when it moves state forward.
func (f *File) AdvanceEntityTimestamp(entityName string, timestamp time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if timestamp.IsZero() {
		return false, fmt.Errorf("timestamp for entity %s must not be zero", entityName)
	}
	timestamp = timestamp.UTC().Truncate(time.Second)

	candidate := make([]types.EntityState, len(f.entities))
	copy(candidate, f.entities)

	found := false
	for i := range candidate {
		if candidate[i].Entity == entityName {
			current, err := candidate[i].GetLastRunTime()
			if err != nil {
				return false, fmt.Errorf("invalid current timestamp for entity %s: %w", entityName, err)
			}
			if !timestamp.After(current) {
				return false, nil
			}
			candidate[i].SetLastRunTime(timestamp)
			found = true
			break
		}
	}

	if !found {
		return false, fmt.Errorf("entity not found: %s", entityName)
	}

	newETag, remoteCommitted, err := f.save(candidate)
	if err == nil || remoteCommitted {
		f.entities = candidate
		if f.s3 != nil {
			f.s3ETag = newETag
		}
	}
	if err != nil {
		return remoteCommitted, err
	}
	return true, nil
}

// save writes the state to disk atomically and uploads to S3 if configured
func (f *File) save(entities []types.EntityState) (newETag *string, remoteCommitted bool, retErr error) {
	// Sort entities by name for consistent output
	sorted := make([]types.EntityState, len(entities))
	copy(sorted, entities)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Entity < sorted[j].Entity
	})

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal state: %w", err)
	}

	if f.s3 != nil && f.s3Key != "" {
		etag, err := f.s3.UploadBytesCAS(context.Background(), f.s3Key, data, f.s3ETag)
		if err != nil {
			return nil, false, fmt.Errorf("failed to upload state to S3 (key=%s): %w", f.s3Key, err)
		}
		newETag = &etag
		remoteCommitted = true
	}

	if err := replaceFileAtomic(f.path, data); err != nil {
		return newETag, remoteCommitted, fmt.Errorf("failed to save local state file: %w", err)
	}

	return newETag, remoteCommitted, nil
}

func replaceFileAtomic(path string, data []byte) (retErr error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmp != nil {
			retErr = errors.Join(retErr, tmp.Close())
		}
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			retErr = errors.Join(retErr, err)
		}
	}()

	if info, err := os.Stat(path); err == nil {
		if err := tmp.Chmod(info.Mode().Perm()); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	closeErr := tmp.Close()
	tmp = nil
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

// GetSQLPath returns the path to the SQL file for an entity
func (f *File) GetSQLPath(sqlDir, entityName string) (string, error) {
	if err := validateEntityName(entityName); err != nil {
		return "", err
	}
	path := filepath.Join(sqlDir, entityName+".sql")
	base, err := filepath.Abs(sqlDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve SQL directory: %w", err)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve SQL path: %w", err)
	}
	rel, err := filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("SQL path escapes configured directory for entity %q", entityName)
	}
	return path, nil
}

// ValidateSQLFiles checks if SQL files exist for all active entities
func (f *File) ValidateSQLFiles(sqlDir string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var missing []string
	for _, e := range f.entities {
		if e.Active {
			sqlPath, err := f.GetSQLPath(sqlDir, e.Entity)
			if err != nil {
				return err
			}
			info, err := os.Stat(sqlPath)
			if os.IsNotExist(err) {
				missing = append(missing, e.Entity)
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to access SQL file %s: %w", sqlPath, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("SQL path is not a regular file: %s", sqlPath)
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
