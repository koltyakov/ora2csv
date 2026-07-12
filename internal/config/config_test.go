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

func TestConfig_ConnectionStringEscapesCredentials(t *testing.T) {
	cfg := &Config{
		DBUser:     "test@user",
		DBPassword: "p@ss:word/with#chars",
		DBHost:     "testhost",
		DBPort:     1521,
		DBService:  "ORCL",
	}

	want := "oracle://test%40user:p%40ss%3Aword%2Fwith%23chars@testhost:1521/ORCL"
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

	t.Run("whitespace db_user", func(t *testing.T) {
		cfg := *validCfg
		cfg.DBUser = "   "
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for whitespace db_user")
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

	t.Run("negative watermark lag", func(t *testing.T) {
		cfg := *validCfg
		cfg.WatermarkLag = -time.Second
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative watermark lag")
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
		if _, err := os.Stat(exportDir); !os.IsNotExist(err) {
			t.Error("ValidatePaths() created export directory")
		}
		if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
			t.Error("ValidatePaths() created state directory")
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

	t.Run("export_dir is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		sqlDir := tmpDir + "/sql"
		exportFile := tmpDir + "/export"
		if err := os.Mkdir(sqlDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exportFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg := &Config{SQLDir: sqlDir, ExportDir: exportFile, StateFile: tmpDir + "/state.json"}
		if err := cfg.ValidatePaths(); err == nil {
			t.Error("expected error when export_dir is a file")
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

func TestValidateDirTarget(t *testing.T) {
	t.Run("existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := validateDirTarget(tmpDir)
		if err != nil {
			t.Errorf("validateDirTarget() error = %v", err)
		}
	})

	t.Run("accepts missing directory without creating it", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := tmpDir + "/new/deep/path"
		err := validateDirTarget(newDir)
		if err != nil {
			t.Errorf("validateDirTarget() error = %v", err)
		}
		if _, err := os.Stat(newDir); !os.IsNotExist(err) {
			t.Error("validateDirTarget() created the directory")
		}
	})
}
