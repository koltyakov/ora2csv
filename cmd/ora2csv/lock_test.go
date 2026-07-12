package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireStateLock_SerializesAndReleases(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	first, err := acquireStateLock(statePath)
	if err != nil {
		t.Fatalf("first acquireStateLock() error: %v", err)
	}

	second, err := acquireStateLock(statePath)
	if !errors.Is(err, errStateLocked) {
		t.Fatalf("second acquireStateLock() error = %v, want errStateLocked", err)
	}
	if second != nil {
		t.Fatal("contended acquire returned a lock")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}

	third, err := acquireStateLock(statePath)
	if err != nil {
		t.Fatalf("third acquireStateLock() error: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("third Close() error: %v", err)
	}
	if _, err := os.Stat(statePath + ".lock"); err != nil {
		t.Fatalf("persistent lock sidecar missing: %v", err)
	}
}

func TestAcquireStateLock_NormalizesStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	first, err := acquireStateLock(statePath)
	if err != nil {
		t.Fatalf("first acquireStateLock() error: %v", err)
	}
	defer func() {
		if err := first.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	}()

	equivalent := filepath.Join(tmpDir, ".", "state.json")
	if _, err := acquireStateLock(equivalent); !errors.Is(err, errStateLocked) {
		t.Fatalf("equivalent path error = %v, want errStateLocked", err)
	}
}

func TestAcquireStateLock_MissingParent(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "missing", "state.json")
	if _, err := acquireStateLock(statePath); err == nil || errors.Is(err, errStateLocked) {
		t.Fatalf("acquireStateLock() error = %v, want filesystem error", err)
	}
}
