package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestRunExport_DryRunHasNoRemoteOrFilesystemSideEffects(t *testing.T) {
	if exportCmd.Parent() == nil {
		rootCmd.AddCommand(exportCmd)
	}
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
	rootCmd.SetArgs([]string{
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
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
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
