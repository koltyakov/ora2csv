package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
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
}

// Load creates a new Config from environment variables and defaults
func Load() *Config {
	v := viper.New()

	// Set defaults
	v.SetDefault("db_host", DefaultDBHost)
	v.SetDefault("db_port", DefaultDBPort)
	v.SetDefault("db_service", DefaultDBService)
	v.SetDefault("db_user", DefaultDBUser)
	v.SetDefault("state_file", DefaultStateFile)
	v.SetDefault("sql_dir", DefaultSQLDir)
	v.SetDefault("export_dir", DefaultExportDir)
	v.SetDefault("days_back", DefaultDaysBack)
	v.SetDefault("connect_timeout", DefaultConnectTimeoutSecs)
	v.SetDefault("query_timeout", DefaultQueryTimeoutSecs)
	v.SetDefault("dry_run", false)
	v.SetDefault("verbose", false)

	// Enable environment variable reading
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()

	// Bind specific env vars that don't follow the prefix pattern
	v.BindEnv("db_password", EnvDBPassword)

	cfg := &Config{
		DBUser:          v.GetString("db_user"),
		DBPassword:      v.GetString("db_password"),
		DBHost:          v.GetString("db_host"),
		DBPort:          v.GetInt("db_port"),
		DBService:       v.GetString("db_service"),
		StateFile:       v.GetString("state_file"),
		SQLDir:          v.GetString("sql_dir"),
		ExportDir:       v.GetString("export_dir"),
		DefaultDaysBack: v.GetInt("days_back"),
		DryRun:          v.GetBool("dry_run"),
		Verbose:         v.GetBool("verbose"),
		ConnectTimeout:  time.Duration(v.GetInt("connect_timeout")) * time.Second,
		QueryTimeout:    time.Duration(v.GetInt("query_timeout")) * time.Second,
	}

	return cfg
}

// BindFlags binds Cobra flags to the config
func (c *Config) BindFlags(v *viper.Viper, flags map[string]interface{}) {
	for name, value := range flags {
		v.SetDefault(name, value)
	}
}

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
	return nil
}

// ConnectionString returns the Oracle connection string for go-ora v2
// Format: oracle://user:password@host:port/service
func (c *Config) ConnectionString() string {
	return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBService)
}

// EnsureDirs creates necessary directories if they don't exist
func (c *Config) EnsureDirs() error {
	if err := os.MkdirAll(c.ExportDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	return nil
}
