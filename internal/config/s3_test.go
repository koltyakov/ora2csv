package config

import (
	"testing"
)

func TestS3Config_Validate(t *testing.T) {
	t.Run("empty bucket is valid", func(t *testing.T) {
		cfg := &S3Config{}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("with bucket", func(t *testing.T) {
		cfg := &S3Config{Bucket: "test-bucket"}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("normalizes prefix with leading slash", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
			Prefix: "/exports",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Prefix != "exports/" {
			t.Errorf("Prefix = %q, want %q", cfg.Prefix, "exports/")
		}
	})

	t.Run("normalizes prefix with trailing slash", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
			Prefix: "exports/",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Prefix != "exports/" {
			t.Errorf("Prefix = %q, want %q", cfg.Prefix, "exports/")
		}
	})

	t.Run("normalizes prefix with both slashes", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
			Prefix: "/exports/",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Prefix != "exports/" {
			t.Errorf("Prefix = %q, want %q", cfg.Prefix, "exports/")
		}
	})

	t.Run("normalizes nested prefix", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
			Prefix: "/data/exports/",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Prefix != "data/exports/" {
			t.Errorf("Prefix = %q, want %q", cfg.Prefix, "data/exports/")
		}
	})

	t.Run("empty prefix remains empty", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
			Prefix: "",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v", err)
		}
		if cfg.Prefix != "" {
			t.Errorf("Prefix = %q, want empty string", cfg.Prefix)
		}
	})
}

func TestS3Config_Key(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *S3Config
		filename string
		want     string
	}{
		{
			name:     "no prefix",
			cfg:      &S3Config{Prefix: ""},
			filename: "test.csv",
			want:     "test.csv",
		},
		{
			name:     "with prefix",
			cfg:      &S3Config{Prefix: "exports/"},
			filename: "test.csv",
			want:     "exports/test.csv",
		},
		{
			name:     "with nested prefix",
			cfg:      &S3Config{Prefix: "data/exports/"},
			filename: "entity.csv",
			want:     "data/exports/entity.csv",
		},
		{
			name:     "filename with path",
			cfg:      &S3Config{Prefix: "exports/"},
			filename: "folder/test.csv",
			want:     "exports/folder/test.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Key(tt.filename)
			if got != tt.want {
				t.Errorf("Key() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestS3Config_StateKey(t *testing.T) {
	tests := []struct {
		name string
		cfg  *S3Config
		want string
	}{
		{
			name: "no prefix",
			cfg:  &S3Config{Prefix: ""},
			want: "state.json",
		},
		{
			name: "with prefix",
			cfg:  &S3Config{Prefix: "exports/"},
			want: "exports/state.json",
		},
		{
			name: "with nested prefix",
			cfg:  &S3Config{Prefix: "data/ora2csv/"},
			want: "data/ora2csv/state.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.StateKey()
			if got != tt.want {
				t.Errorf("StateKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestS3Config_IsMinIO(t *testing.T) {
	tests := []struct {
		name string
		cfg  *S3Config
		want bool
	}{
		{
			name: "no endpoint",
			cfg:  &S3Config{Endpoint: ""},
			want: false,
		},
		{
			name: "AWS endpoint",
			cfg:  &S3Config{Endpoint: "https://s3.amazonaws.com"},
			want: false,
		},
		{
			name: "AWS regional endpoint",
			cfg:  &S3Config{Endpoint: "https://s3.us-east-1.amazonaws.com"},
			want: false,
		},
		{
			name: "MinIO endpoint",
			cfg:  &S3Config{Endpoint: "http://localhost:9000"},
			want: true,
		},
		{
			name: "Wasabi endpoint",
			cfg:  &S3Config{Endpoint: "https://s3.wasabisys.com"},
			want: true,
		},
		{
			name: "custom S3-compatible endpoint",
			cfg:  &S3Config{Endpoint: "https://minio.example.com"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IsMinIO()
			if got != tt.want {
				t.Errorf("IsMinIO() = %v, want %v", got, tt.want)
			}
		})
	}
}
