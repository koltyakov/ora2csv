package config

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestFromCommand_NullValue(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv(EnvNullValue, `\N`)
		cmd := &cobra.Command{}
		cmd.Flags().String("null-value", "", "")
		cfg, err := FromCommand(cmd)
		if err != nil {
			t.Fatalf("FromCommand() error: %v", err)
		}
		if cfg.NullValue != `\N` {
			t.Fatalf("NullValue = %q, want \\N", cfg.NullValue)
		}
	})

	t.Run("flag overrides environment", func(t *testing.T) {
		t.Setenv(EnvNullValue, "ENV_NULL")
		cmd := &cobra.Command{}
		cmd.Flags().String("null-value", "", "")
		if err := cmd.Flags().Set("null-value", "FLAG_NULL"); err != nil {
			t.Fatalf("Set() error: %v", err)
		}
		cfg, err := FromCommand(cmd)
		if err != nil {
			t.Fatalf("FromCommand() error: %v", err)
		}
		if cfg.NullValue != "FLAG_NULL" {
			t.Fatalf("NullValue = %q, want FLAG_NULL", cfg.NullValue)
		}
	})
}

func TestFromCommand_S3UploadSettings(t *testing.T) {
	t.Setenv(EnvS3UploadTimeout, "12m")
	t.Setenv(EnvS3PartSize, "10485760")
	t.Setenv(EnvS3Concurrency, "8")
	cmd := &cobra.Command{}
	cmd.Flags().Duration("s3-upload-timeout", 5*time.Minute, "")
	cmd.Flags().Int64("s3-part-size", DefaultS3PartSize, "")
	cmd.Flags().Int("s3-concurrency", DefaultS3Concurrency, "")
	cfg, err := FromCommand(cmd)
	if err != nil {
		t.Fatalf("FromCommand() error: %v", err)
	}
	if cfg.S3.UploadTimeout != 12*time.Minute || cfg.S3.PartSize != 10485760 || cfg.S3.Concurrency != 8 {
		t.Fatalf("S3 settings = (%v, %d, %d)", cfg.S3.UploadTimeout, cfg.S3.PartSize, cfg.S3.Concurrency)
	}
}
