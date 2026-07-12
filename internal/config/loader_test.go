package config

import (
	"testing"

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
