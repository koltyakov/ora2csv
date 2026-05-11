package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	// Database connection
	DBUser     string `mapstructure:"db_user"`
	DBPassword string `mapstructure:"db_password"`
	DBHost     string `mapstructure:"db_host"`
	DBPort     int    `mapstructure:"db_port"`
	DBService  string `mapstructure:"db_service"`

	// Paths
	StateFile string `mapstructure:"state_file"`
	SQLDir    string `mapstructure:"sql_dir"`
	ExportDir string `mapstructure:"export_dir"`

	// Behavior
	DefaultDaysBack int  `mapstructure:"days_back"`
	DryRun          bool `mapstructure:"dry_run"`
	Verbose         bool `mapstructure:"verbose"`

	// Timeouts
	ConnectTimeout time.Duration `mapstructure:"-"`
	QueryTimeout   time.Duration `mapstructure:"-"`

	// S3 destination
	S3 S3Config `mapstructure:",squash"`
}

// ConnectionString returns the Oracle connection string for go-ora v2
// Format: oracle://user:password@host:port/service
func (c *Config) ConnectionString() string {
	u := &url.URL{
		Scheme: "oracle",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort)),
		Path:   "/" + c.DBService,
	}
	return u.String()
}

// EnsureDirs creates necessary directories if they don't exist
func (c *Config) EnsureDirs() error {
	if err := os.MkdirAll(c.ExportDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	return nil
}
