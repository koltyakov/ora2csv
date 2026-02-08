package testutil

import (
	"os"
	"path/filepath"
	"testing/fstest"
	"time"

	"github.com/koltyakov/ora2csv/internal/config"
	"github.com/koltyakov/ora2csv/pkg/types"
)

// TempDir creates a temporary directory for testing
func TempDir(t TB) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

// TB is the interface shared by testing.T and testing.B
type TB interface {
	TempDir() string
	Cleanup(func())
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
}

// NewTestConfig returns a test configuration with temporary directories
func NewTestConfig(t TB) *config.Config {
	t.Helper()
	tmpDir := TempDir(t)

	return &config.Config{
		DBUser:          "test_user",
		DBPassword:      "test_password",
		DBHost:          "localhost",
		DBPort:          1521,
		DBService:       "TEST",
		StateFile:       filepath.Join(tmpDir, "state.json"),
		SQLDir:          filepath.Join(tmpDir, "sql"),
		ExportDir:       filepath.Join(tmpDir, "export"),
		DefaultDaysBack: 30,
		DryRun:          false,
		Verbose:         true,
		ConnectTimeout:  30 * time.Second,
		QueryTimeout:    5 * time.Minute,
	}
}

// NewTestState returns test entity states
func NewTestState() []types.EntityState {
	return []types.EntityState{
		{
			Entity:      "test.entity1",
			LastRunTime: "2025-01-01T00:00:00",
			Active:      true,
		},
		{
			Entity:      "test.entity2",
			LastRunTime: "",
			Active:      true,
		},
		{
			Entity:      "test.entity3",
			LastRunTime: "2025-01-01T00:00:00",
			Active:      false,
		},
	}
}

// WriteStateFile writes a state file to the given path
func WriteStateFile(path string, entities []types.EntityState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	for i, e := range entities {
		if i > 0 {
			if _, err := f.WriteString(",\n"); err != nil {
				_ = f.Close()
				return err
			}
		}
		if _, err := f.WriteString("  "); err != nil {
			_ = f.Close()
			return err
		}
		if _, err := f.WriteString(`{"entity":"` + e.Entity + `",`); err != nil {
			_ = f.Close()
			return err
		}
		if _, err := f.WriteString(`"lastRunTime":"` + e.LastRunTime + `",`); err != nil {
			_ = f.Close()
			return err
		}
		if _, err := f.WriteString(`"active":` + formatBool(e.Active) + `}`); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// formatBool returns a JSON boolean string
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// CreateTestSQLFiles creates test SQL files in the given directory
func CreateTestSQLFiles(sqlDir string, entities []types.EntityState) error {
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		return err
	}

	for _, e := range entities {
		sqlPath := filepath.Join(sqlDir, e.Entity+".sql")
		sqlContent := `SELECT
  id,
  name,
  TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM ` + e.Entity + `
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY updated ASC`
		if err := os.WriteFile(sqlPath, []byte(sqlContent), 0644); err != nil {
			return err
		}
	}

	return nil
}

// MapFS creates a test filesystem map for testing
func MapFS() fstest.MapFS {
	return fstest.MapFS{
		"state.json": {
			Data: []byte(`[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true}
]`),
		},
		"sql/test.entity1.sql": {
			Data: []byte("SELECT * FROM test.entity1"),
		},
		"sql/test.entity2.sql": {
			Data: []byte("SELECT * FROM test.entity2"),
		},
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEqual fails the test if want != got
func AssertEqual[T comparable](t TB, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("got %v, want %v", got, want)
	}
}
