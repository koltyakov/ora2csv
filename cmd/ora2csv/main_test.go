package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunExport_DryRunHasNoRemoteOrFilesystemSideEffects(t *testing.T) {
	root := newRootCommand("test", "test")
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	sqlDir := filepath.Join(tmpDir, "sql")
	exportDir := filepath.Join(tmpDir, "export")
	if err := os.Mkdir(sqlDir, 0755); err != nil {
		t.Fatalf("Mkdir(sqlDir) error: %v", err)
	}
	stateData := []byte(`[{"entity":"crm.orders","lastRunTime":"","active":true}]`)
	if err := os.WriteFile(statePath, stateData, 0600); err != nil {
		t.Fatalf("WriteFile(state) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sqlDir, "crm.orders.sql"), []byte("SELECT 1"), 0600); err != nil {
		t.Fatalf("WriteFile(SQL) error: %v", err)
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv("ORA2CSV_DB_PASSWORD", "test-password")
	root.SetArgs([]string{
		"export",
		"--state-file", statePath,
		"--sql-dir", sqlDir,
		"--export-dir", exportDir,
		"--dry-run",
		"--s3-bucket", "test-bucket",
		"--s3-endpoint", server.URL,
		"--s3-access-key", "test-key",
		"--s3-secret-key", "test-secret",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("dry-run sent %d remote requests", requests.Load())
	}
	if _, err := os.Stat(exportDir); !os.IsNotExist(err) {
		t.Fatal("dry-run created the export directory")
	}
	if _, err := os.Stat(statePath + ".lock"); !os.IsNotExist(err) {
		t.Fatal("dry-run created a state lock")
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state) error: %v", err)
	}
	if string(after) != string(stateData) {
		t.Fatalf("dry-run changed state to %q", after)
	}
}

func TestCommandsRejectPositionalArguments(t *testing.T) {
	for _, command := range []string{"export", "validate"} {
		t.Run(command, func(t *testing.T) {
			root := newRootCommand("test", "test")
			root.SetArgs([]string{command, "unexpected"})
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), "unknown command \"unexpected\"") {
				t.Fatalf("Execute() error = %v, want positional argument rejection", err)
			}
		})
	}
}

func TestRootCommandVersion(t *testing.T) {
	root := newRootCommand("1.2.3", "2026-07-12T10:00:00Z")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if stdout.String() != "ora2csv version 1.2.3 (built: 2026-07-12T10:00:00Z)\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
