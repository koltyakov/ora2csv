package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// S3Config holds S3 destination configuration
type S3Config struct {
	Bucket        string        `mapstructure:"s3_bucket"`
	Prefix        string        `mapstructure:"s3_prefix"`
	AccessKey     string        `mapstructure:"s3_access_key"`
	SecretKey     string        `mapstructure:"s3_secret_key"`
	SessionToken  string        `mapstructure:"s3_session_token"`
	Endpoint      string        `mapstructure:"s3_endpoint"` // For MinIO, Wasabi, etc.
	UploadTimeout time.Duration `mapstructure:"-"`
	PartSize      int64         `mapstructure:"s3_part_size"`
	Concurrency   int           `mapstructure:"s3_concurrency"`
}

// Validate checks if S3 configuration is valid
func (c *S3Config) Validate() error {
	if c.Bucket == "" {
		return nil
	}
	if c.UploadTimeout == 0 {
		c.UploadTimeout = DefaultS3UploadTimeoutSecs * time.Second
	}
	if c.PartSize == 0 {
		c.PartSize = DefaultS3PartSize
	}
	if c.Concurrency == 0 {
		c.Concurrency = DefaultS3Concurrency
	}
	if c.UploadTimeout < time.Second || c.UploadTimeout > 24*time.Hour {
		return fmt.Errorf("s3_upload_timeout must be between 1s and 24h")
	}
	if c.PartSize < DefaultS3PartSize || c.PartSize > MaxS3PartSize {
		return fmt.Errorf("s3_part_size must be between %d and %d bytes", DefaultS3PartSize, MaxS3PartSize)
	}
	if c.Concurrency < 1 || c.Concurrency > 100 {
		return fmt.Errorf("s3_concurrency must be between 1 and 100")
	}

	// Clean up prefix - ensure it doesn't start/end with slash
	c.Prefix = strings.Trim(c.Prefix, "/")
	if c.Prefix != "" {
		c.Prefix += "/"
	}

	return nil
}

// Key returns the S3 key for a given filename
func (c *S3Config) Key(filename string) string {
	if c.Prefix == "" {
		return filename
	}
	return filepath.ToSlash(filepath.Join(c.Prefix, filename))
}

// StateKey returns the S3 key for the state file
func (c *S3Config) StateKey() string {
	return c.Key("state.json")
}

// IsMinIO returns true if the configuration appears to be for MinIO or similar S3-compatible service
func (c *S3Config) IsMinIO() bool {
	return c.Endpoint != "" && !strings.Contains(c.Endpoint, "amazonaws.com")
}
