package config

import (
	"os"
	"testing"
	"time"
)

func TestConfig_ConnectionString(t *testing.T) {
	cfg := &Config{
		DBUser:     "testuser",
		DBPassword: "testpass",
		DBHost:     "testhost",
		DBPort:     1521,
		DBService:  "TESTSVC",
	}

	want := "oracle://testuser:testpass@testhost:1521/TESTSVC"
	got := cfg.ConnectionString()
	if got != want {
		t.Errorf("ConnectionString() = %q, want %q", got, want)
	}
}

func TestConfig_EnsureDirs(t *testing.T) {
	t.Run("creates export directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		exportDir := tmpDir + "/export/subdir"

		cfg := &Config{
			ExportDir: exportDir,
		}

		err := cfg.EnsureDirs()
		if err != nil {
			t.Fatalf("EnsureDirs() error = %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(exportDir)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if !info.IsDir() {
			t.Error("ExportDir is not a directory")
		}
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Use a path that cannot be created
		cfg := &Config{
			ExportDir: "/dev/null/invalid/path",
		}

		err := cfg.EnsureDirs()
		if err == nil {
			t.Error("expected error for invalid path, got nil")
		}
	})
}

func TestConfig_Validate(t *testing.T) {
	validCfg := &Config{
		DBUser:          "testuser",
		DBPassword:      "testpass",
		DBHost:          "localhost",
		DBPort:          1521,
		DBService:       "ORCL",
		StateFile:       "state.json",
		SQLDir:          "./sql",
		ExportDir:       "./export",
		ConnectTimeout:  30 * time.Second,
		QueryTimeout:    5 * time.Minute,
		DefaultDaysBack: 30,
	}

	t.Run("valid config", func(t *testing.T) {
		err := validCfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("missing db_user", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBUser = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing db_user")
		}
	})

	t.Run("missing db_password", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBPassword = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing db_password")
		}
	})

	t.Run("missing db_host", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBHost = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing db_host")
		}
	})

	t.Run("invalid db_port - zero", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBPort = 0
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for invalid db_port")
		}
	})

	t.Run("invalid db_port - negative", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBPort = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for invalid db_port")
		}
	})

	t.Run("invalid db_port - too large", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBPort = 70000
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for invalid db_port")
		}
	})

	t.Run("valid db_port boundary", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBPort = 65535
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v (port 65535 should be valid)", err)
		}
	})

	t.Run("missing db_service", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBService = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing db_service")
		}
	})

	t.Run("missing state_file", func(t *testing.T) {
		cfg := *validCfg
		cfg.StateFile = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing state_file")
		}
	})

	t.Run("missing sql_dir", func(t *testing.T) {
		cfg := *validCfg
		cfg.SQLDir = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing sql_dir")
		}
	})

	t.Run("missing export_dir", func(t *testing.T) {
		cfg := *validCfg
		cfg.ExportDir = ""
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing export_dir")
		}
	})

	t.Run("connect_timeout too small", func(t *testing.T) {
		cfg := *validCfg
		cfg.ConnectTimeout = 100 * time.Millisecond
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for connect_timeout too small")
		}
	})

	t.Run("connect_timeout too large", func(t *testing.T) {
		cfg := *validCfg
		cfg.ConnectTimeout = 2 * time.Hour
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for connect_timeout too large")
		}
	})

	t.Run("query_timeout too small", func(t *testing.T) {
		cfg := *validCfg
		cfg.QueryTimeout = 100 * time.Millisecond
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for query_timeout too small")
		}
	})

	t.Run("query_timeout too large", func(t *testing.T) {
		cfg := *validCfg
		cfg.QueryTimeout = 25 * time.Hour
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for query_timeout too large")
		}
	})

	t.Run("valid query_timeout boundary", func(t *testing.T) {
		cfg := *validCfg
		cfg.QueryTimeout = 24 * time.Hour
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v (24h should be valid)", err)
		}
	})

	t.Run("days_back negative", func(t *testing.T) {
		cfg := *validCfg
		cfg.DefaultDaysBack = -1
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for negative days_back")
		}
	})

	t.Run("days_back too large", func(t *testing.T) {
		cfg := *validCfg
		cfg.DefaultDaysBack = 4000
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for days_back too large")
		}
	})

	t.Run("valid days_back boundary", func(t *testing.T) {
		cfg := *validCfg
		cfg.DefaultDaysBack = 3650
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v (3650 should be valid)", err)
		}
	})

	t.Run("valid days_back zero", func(t *testing.T) {
		cfg := *validCfg
		cfg.DefaultDaysBack = 0
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v (0 should be valid)", err)
		}
	})
}

func TestConfig_ValidatePaths(t *testing.T) {
	t.Run("valid paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		sqlDir := tmpDir + "/sql"
		exportDir := tmpDir + "/export"
		stateDir := tmpDir + "/state"

		// Create sql directory
		if err := os.MkdirAll(sqlDir, 0755); err != nil {
			t.Fatal(err)
		}

		cfg := &Config{
			SQLDir:    sqlDir,
			ExportDir: exportDir,
			StateFile: stateDir + "/state.json",
		}

		err := cfg.ValidatePaths()
		if err != nil {
			t.Errorf("ValidatePaths() error = %v", err)
		}
	})

	t.Run("sql_dir does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{
			SQLDir:    tmpDir + "/nonexistent",
			ExportDir: tmpDir + "/export",
			StateFile: tmpDir + "/state.json",
		}

		err := cfg.ValidatePaths()
		if err == nil {
			t.Error("expected error for nonexistent sql_dir")
		}
	})

	t.Run("sql_dir is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		sqlFile := tmpDir + "/notadir"
		if err := os.WriteFile(sqlFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := &Config{
			SQLDir:    sqlFile,
			ExportDir: tmpDir + "/export",
			StateFile: tmpDir + "/state.json",
		}

		err := cfg.ValidatePaths()
		if err == nil {
			t.Error("expected error when sql_dir is a file")
		}
	})
}

func TestValidateDirReadable(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := validateDirReadable(tmpDir)
		if err != nil {
			t.Errorf("validateDirReadable() error = %v", err)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		err := validateDirReadable("/nonexistent/path")
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := tmpDir + "/file"
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		err := validateDirReadable(filePath)
		if err == nil {
			t.Error("expected error when path is a file")
		}
	})
}

func TestValidateDirWritable(t *testing.T) {
	t.Run("existing writable directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := validateDirWritable(tmpDir)
		if err != nil {
			t.Errorf("validateDirWritable() error = %v", err)
		}
	})

	t.Run("creates new directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := tmpDir + "/new/deep/path"
		err := validateDirWritable(newDir)
		if err != nil {
			t.Errorf("validateDirWritable() error = %v", err)
		}

		// Verify it was created
		info, err := os.Stat(newDir)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if !info.IsDir() {
			t.Error("Path is not a directory")
		}
	})
}

func TestDirPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"unix path", "/home/user/file.json", "/home/user"},
		{"windows path", `C:\Users\user\file.json`, `C:\Users\user`},
		{"relative path", "dir/file.json", "dir"},
		{"no directory", "file.json", "."},
		{"nested path", "a/b/c/d/file.json", "a/b/c/d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dirPath(tt.path)
			if got != tt.expected {
				t.Errorf("dirPath() = %q, want %q", got, tt.expected)
			}
		})
	}
}
