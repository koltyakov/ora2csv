package config

import (
	"fmt"
	"os"
	"time"
)

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DBUser == "" {
		return fmt.Errorf("db_user is required")
	}
	if c.DBPassword == "" {
		return fmt.Errorf("db_password is required (set %s env var)", EnvDBPassword)
	}
	if c.DBHost == "" {
		return fmt.Errorf("db_host is required")
	}
	if c.DBPort <= 0 || c.DBPort > 65535 {
		return fmt.Errorf("db_port must be between 1 and 65535")
	}
	if c.DBService == "" {
		return fmt.Errorf("db_service is required")
	}
	if c.StateFile == "" {
		return fmt.Errorf("state_file is required")
	}
	if c.SQLDir == "" {
		return fmt.Errorf("sql_dir is required")
	}
	if c.ExportDir == "" {
		return fmt.Errorf("export_dir is required")
	}

	// Validate timeouts
	if c.ConnectTimeout < time.Second || c.ConnectTimeout > time.Hour {
		return fmt.Errorf("connect_timeout must be between 1s and 1h")
	}
	if c.QueryTimeout < time.Second || c.QueryTimeout > 24*time.Hour {
		return fmt.Errorf("query_timeout must be between 1s and 24h")
	}

	// Validate days_back
	if c.DefaultDaysBack < 0 || c.DefaultDaysBack > 3650 {
		return fmt.Errorf("days_back must be between 0 and 3650")
	}

	// Validate S3 configuration
	if err := c.S3.Validate(); err != nil {
		return err
	}

	return nil
}

// ValidatePaths checks if paths are accessible
func (c *Config) ValidatePaths() error {
	// Check SQL directory exists and is readable
	if err := validateDirReadable(c.SQLDir); err != nil {
		return fmt.Errorf("sql_dir validation failed: %w", err)
	}

	// Check export directory can be created/written
	if err := validateDirWritable(c.ExportDir); err != nil {
		return fmt.Errorf("export_dir validation failed: %w", err)
	}

	// Check state file parent directory is writable
	stateDir := dirPath(c.StateFile)
	if stateDir != "." {
		if err := validateDirWritable(stateDir); err != nil {
			return fmt.Errorf("state file directory validation failed: %w", err)
		}
	}

	return nil
}

// validateDirReadable checks if a directory exists and is readable
func validateDirReadable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}

	// Try to read directory
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("directory not readable: %w", err)
	}
	f.Close()

	return nil
}

// validateDirWritable checks if a directory can be written to
func validateDirWritable(path string) error {
	// If directory doesn't exist, try to create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("cannot create directory: %w", err)
		}
		return nil
	}

	// Directory exists, check if writable
	testFile := fmt.Sprintf("%s/.write_test_%d", path, time.Now().UnixNano())
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// dirPath returns the directory path of a file path
func dirPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
