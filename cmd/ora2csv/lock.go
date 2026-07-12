package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gofrs/flock"
)

var errStateLocked = errors.New("state file is locked")

func acquireStateLock(statePath string) (*flock.Flock, error) {
	absolutePath, err := filepath.Abs(statePath)
	if err != nil {
		return nil, fmt.Errorf("resolve state file path: %w", err)
	}

	lock := flock.New(absolutePath + ".lock")
	locked, err := lock.TryLock()
	if err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("lock state file %s: %w", absolutePath, err)
	}
	if !locked {
		_ = lock.Close()
		return nil, fmt.Errorf("%w: another ora2csv export is using %s", errStateLocked, absolutePath)
	}
	return lock, nil
}
