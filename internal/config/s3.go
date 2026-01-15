package config

import (
	"path/filepath"
	"strings"
)

// S3Config holds S3 destination configuration
type S3Config struct {
	Bucket       string `mapstructure:"s3_bucket"`
	Prefix       string `mapstructure:"s3_prefix"`
	AccessKey    string `mapstructure:"s3_access_key"`
	SecretKey    string `mapstructure:"s3_secret_key"`
	SessionToken string `mapstructure:"s3_session_token"`
	Endpoint     string `mapstructure:"s3_endpoint"` // For MinIO, Wasabi, etc.
}

// Validate checks if S3 configuration is valid
func (c *S3Config) Validate() error {
	if c.Bucket == "" {
		return nil
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
