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
		{"watermark-lag", "watermark_lag"},
		{"null-value", "null_value"},
		// S3 flags (note: auth flags kept for non-AWS S3-compatible services)
		{"s3-bucket", "s3_bucket"},
		{"s3-prefix", "s3_prefix"},
		{"s3-access-key", "s3_access_key"},
		{"s3-secret-key", "s3_secret_key"},
		{"s3-session-token", "s3_session_token"},
		{"s3-endpoint", "s3_endpoint"},
		{"s3-upload-timeout", "s3_upload_timeout"},
		{"s3-part-size", "s3_part_size"},
		{"s3-concurrency", "s3_concurrency"},
		{"s3-allow-insecure-endpoint", "s3_allow_insecure_endpoint"},
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
	if err := v.BindEnv("watermark_lag", EnvWatermarkLag); err != nil {
		return nil, fmt.Errorf("failed to bind watermark lag env var: %w", err)
	}
	if err := v.BindEnv("null_value", EnvNullValue); err != nil {
		return nil, fmt.Errorf("failed to bind null value env var: %w", err)
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
	if err := v.BindEnv("s3_upload_timeout", EnvS3UploadTimeout); err != nil {
		return nil, fmt.Errorf("failed to bind S3 upload timeout env var: %w", err)
	}
	if err := v.BindEnv("s3_part_size", EnvS3PartSize); err != nil {
		return nil, fmt.Errorf("failed to bind S3 part size env var: %w", err)
	}
	if err := v.BindEnv("s3_concurrency", EnvS3Concurrency); err != nil {
		return nil, fmt.Errorf("failed to bind S3 concurrency env var: %w", err)
	}
	if err := v.BindEnv("s3_allow_insecure_endpoint", EnvS3AllowInsecure); err != nil {
		return nil, fmt.Errorf("failed to bind insecure S3 endpoint env var: %w", err)
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
	v.SetDefault("watermark_lag", DefaultWatermarkLagSecs*time.Second)
	v.SetDefault("null_value", DefaultNullValue)
	v.SetDefault("s3_upload_timeout", DefaultS3UploadTimeoutSecs*time.Second)
	v.SetDefault("s3_part_size", DefaultS3PartSize)
	v.SetDefault("s3_concurrency", DefaultS3Concurrency)
	v.SetDefault("s3_allow_insecure_endpoint", false)

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
	result.WatermarkLag = v.GetDuration("watermark_lag")
	result.S3.UploadTimeout = v.GetDuration("s3_upload_timeout")

	return result, nil
}
