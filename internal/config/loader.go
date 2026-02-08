package config

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// FromCommand loads configuration from cobra command flags and environment variables
func FromCommand(cmd *cobra.Command) (*Config, error) {
	v := viper.New()

	// Bind flags to viper
	flags := []struct {
		name string
		key  string
	}{
		{"db-host", "db_host"},
		{"db-port", "db_port"},
		{"db-service", "db_service"},
		{"db-user", "db_user"},
		{"state-file", "state_file"},
		{"sql-dir", "sql_dir"},
		{"export-dir", "export_dir"},
		{"days-back", "days_back"},
		{"dry-run", "dry_run"},
		{"verbose", "verbose"},
		{"connect-timeout", "connect_timeout"},
		{"query-timeout", "query_timeout"},
		// S3 flags (note: auth flags kept for non-AWS S3-compatible services)
		{"s3-bucket", "s3_bucket"},
		{"s3-prefix", "s3_prefix"},
		{"s3-access-key", "s3_access_key"},
		{"s3-secret-key", "s3_secret_key"},
		{"s3-session-token", "s3_session_token"},
		{"s3-endpoint", "s3_endpoint"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.name)
		if flag != nil {
			_ = v.BindPFlag(f.key, flag)
		}
	}

	// Enable environment variable reading
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	if err := v.BindEnv("db_password", EnvDBPassword); err != nil {
		return nil, fmt.Errorf("failed to bind db password env var: %w", err)
	}

	// S3 environment variable bindings
	if err := v.BindEnv("s3_bucket", EnvS3Bucket); err != nil {
		return nil, fmt.Errorf("failed to bind s3 bucket env var: %w", err)
	}
	if err := v.BindEnv("s3_prefix", EnvS3Prefix); err != nil {
		return nil, fmt.Errorf("failed to bind s3 prefix env var: %w", err)
	}
	if err := v.BindEnv("s3_endpoint", EnvS3Endpoint); err != nil {
		return nil, fmt.Errorf("failed to bind s3 endpoint env var: %w", err)
	}

	// Set defaults from config package
	v.SetDefault("db_host", DefaultDBHost)
	v.SetDefault("db_port", DefaultDBPort)
	v.SetDefault("db_service", DefaultDBService)
	v.SetDefault("db_user", DefaultDBUser)
	v.SetDefault("state_file", DefaultStateFile)
	v.SetDefault("sql_dir", DefaultSQLDir)
	v.SetDefault("export_dir", DefaultExportDir)
	v.SetDefault("days_back", DefaultDaysBack)
	v.SetDefault("dry_run", false)
	v.SetDefault("verbose", false)
	v.SetDefault("connect_timeout", DefaultConnectTimeoutSecs*time.Second)
	v.SetDefault("query_timeout", DefaultQueryTimeoutSecs*time.Second)

	// S3 defaults
	// No defaults - using AWS SDK default region and credential chain

	// Unmarshal to config
	result := &Config{}
	if err := v.Unmarshal(result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set durations from duration flags
	result.ConnectTimeout = v.GetDuration("connect_timeout")
	result.QueryTimeout = v.GetDuration("query_timeout")

	return result, nil
}
