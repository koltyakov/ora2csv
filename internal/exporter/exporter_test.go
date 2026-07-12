package exporter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koltyakov/ora2csv/internal/config"
	"github.com/koltyakov/ora2csv/internal/db"
	"github.com/koltyakov/ora2csv/internal/logging"
	"github.com/koltyakov/ora2csv/internal/state"
)

func newTestExporter(t *testing.T, rows db.RowScanner) (*Exporter, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	sqlDir := filepath.Join(tmpDir, "sql")
	exportDir := filepath.Join(tmpDir, "export")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		t.Fatalf("MkdirAll(sqlDir) error: %v", err)
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		t.Fatalf("MkdirAll(exportDir) error: %v", err)
	}
	if err := os.WriteFile(statePath, []byte(`[{"entity":"crm.orders","lastRunTime":"2025-01-01T00:00:00","active":true}]`), 0600); err != nil {
		t.Fatalf("WriteFile(state) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sqlDir, "crm.orders.sql"), []byte("SELECT id FROM orders"), 0600); err != nil {
		t.Fatalf("WriteFile(SQL) error: %v", err)
	}
	st, err := state.Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("state.Load() error: %v", err)
	}
	database := db.NewMockDB()
	database.QueryFunc = func(ctx context.Context, query string, args map[string]interface{}) (db.RowScanner, error) {
		return rows, nil
	}
	cfg := &config.Config{
		StateFile:    statePath,
		SQLDir:       sqlDir,
		ExportDir:    exportDir,
		QueryTimeout: time.Minute,
	}
	return New(cfg, database, st, logging.New(false), nil), statePath, exportDir
}

func TestExporterRun_CommitsOutputAndState(t *testing.T) {
	rows := db.NewMockRowScanner([]string{"id", "name"}, [][]string{{"1", "Alice"}})
	exp, statePath, exportDir := newTestExporter(t, rows)

	result, err := exp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.SuccessCount != 1 || result.Results[0].RowCount != 1 {
		t.Fatalf("result = %+v", result)
	}
	outputPath := filepath.Join(exportDir, "crm.orders__2025-01-01T00-00-00.csv")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error: %v", err)
	}
	if string(data) != "id,name\n1,Alice\n" {
		t.Fatalf("output = %q", data)
	}
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state) error: %v", err)
	}
	if strings.Contains(string(stateData), `"lastRunTime": "2025-01-01T00:00:00"`) {
		t.Fatal("state timestamp was not advanced")
	}
	matches, err := filepath.Glob(filepath.Join(exportDir, ".*.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary outputs remain: %v, %v", matches, err)
	}
}

func TestExporterRun_AbortPreservesExistingOutput(t *testing.T) {
	rows := db.NewMockRowScanner([]string{"id"}, [][]string{{"1"}})
	rows.ScanErr = errors.New("scan failed")
	exp, statePath, exportDir := newTestExporter(t, rows)
	outputPath := filepath.Join(exportDir, "crm.orders__2025-01-01T00-00-00.csv")
	if err := os.WriteFile(outputPath, []byte("existing\n"), 0600); err != nil {
		t.Fatalf("WriteFile(existing output) error: %v", err)
	}

	result, err := exp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.FailedCount != 1 {
		t.Fatalf("FailedCount = %d, want 1", result.FailedCount)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error: %v", err)
	}
	if string(data) != "existing\n" {
		t.Fatalf("existing output changed to %q", data)
	}
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state) error: %v", err)
	}
	if !strings.Contains(string(stateData), "2025-01-01T00:00:00") {
		t.Fatal("failed export advanced state")
	}
}

func TestExporterRun_ZeroRowsPublishesNoFile(t *testing.T) {
	rows := db.NewMockRowScanner([]string{"id"}, nil)
	exp, _, exportDir := newTestExporter(t, rows)

	result, err := exp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.SuccessCount != 1 || result.Results[0].RowCount != 0 {
		t.Fatalf("result = %+v", result)
	}
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("zero-row export created files: %v", entries)
	}
}
