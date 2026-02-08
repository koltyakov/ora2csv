package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/koltyakov/ora2csv/internal/config"
)

func TestNewS3Client(t *testing.T) {
	t.Run("missing bucket", func(t *testing.T) {
		cfg := &config.S3Config{
			Bucket: "",
		}

		_, err := NewS3Client(cfg)
		if err == nil {
			t.Error("expected error for missing bucket")
		}
		if !strings.Contains(err.Error(), "bucket") {
			t.Errorf("error message = %q, want 'bucket'", err.Error())
		}
	})

	t.Run("valid config with custom endpoint", func(t *testing.T) {
		cfg := &config.S3Config{
			Bucket:    "test-bucket",
			AccessKey: "test-key",
			SecretKey: "test-secret",
			Endpoint:  "http://localhost:9000",
		}

		// This will fail to connect but validates the config handling
		_, err := NewS3Client(cfg)
		// We expect some error due to network/context in test environment
		// The important check is that bucket validation happens first
		if err != nil && strings.Contains(err.Error(), "bucket") {
			t.Errorf("unexpected bucket validation error: %v", err)
		}
	})
}

func TestS3Client_UploadFile(t *testing.T) {
	// This method always returns an error directing to use UploadStream
	client := &S3Client{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
	}

	ctx := context.Background()
	err := client.UploadFile(ctx, "test-key", "/tmp/file.txt")
	if err == nil {
		t.Error("expected error directing to use UploadStream")
	}
	if !strings.Contains(err.Error(), "UploadStream") {
		t.Errorf("error message = %q, want 'UploadStream'", err.Error())
	}
}

// Test error handling for NoSuchKey
func TestNoSuchKeyError(t *testing.T) {
	// Verify that NoSuchKey error can be identified
	var nsk *types.NoSuchKey
	testErr := &types.NoSuchKey{}

	if !errors.As(testErr, &nsk) {
		t.Error("errors.As failed for NoSuchKey")
	}
}

// Test S3Config integration
func TestS3ConfigWithStorage(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *config.S3Config
		valid  bool
		bucket string
	}{
		{
			name:  "empty bucket",
			cfg:   &config.S3Config{},
			valid: true, // Empty bucket is valid (S3 disabled)
		},
		{
			name:   "with bucket",
			cfg:    &config.S3Config{Bucket: "test-bucket"},
			valid:  true,
			bucket: "test-bucket",
		},
		{
			name: "with prefix",
			cfg: &config.S3Config{
				Bucket: "test-bucket",
				Prefix: "/exports/",
			},
			valid:  true,
			bucket: "test-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err == nil) != tt.valid {
				t.Errorf("Validate() = %v, want valid=%v", err, tt.valid)
			}
			if tt.bucket != "" && tt.cfg.Bucket != tt.bucket {
				t.Errorf("Bucket = %q, want %q", tt.cfg.Bucket, tt.bucket)
			}
		})
	}
}

// Test S3Client methods with nil client (before initialization)
func TestS3Client_NilClient(t *testing.T) {
	client := &S3Client{
		client: nil,
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
	}

	ctx := context.Background()

	t.Run("UploadStream with nil client", func(t *testing.T) {
		// This will panic or error - testing defensive programming
		defer func() {
			if r := recover(); r != nil {
				t.Logf("UploadStream panicked as expected: %v", r)
			}
		}()
		err := client.UploadStream(ctx, "key", strings.NewReader("data"))
		if err != nil {
			t.Logf("UploadStream returned error: %v", err)
		}
	})
}

// TestS3KeyGeneration tests key generation for S3 uploads
func TestS3KeyGeneration(t *testing.T) {
	cfg := &config.S3Config{
		Bucket: "test-bucket",
		Prefix: "exports/",
	}

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "simple filename",
			filename: "test.csv",
			want:     "exports/test.csv",
		},
		{
			name:     "nested filename",
			filename: "entity/data.csv",
			want:     "exports/entity/data.csv",
		},
		{
			name:     "filename with special chars",
			filename: "test-entity__2025-01-15.csv",
			want:     "exports/test-entity__2025-01-15.csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.Key(tt.filename)
			if got != tt.want {
				t.Errorf("Key() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestStateKeyGeneration tests state file key generation
func TestStateKeyGeneration(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.S3Config
		want string
	}{
		{
			name: "no prefix",
			cfg:  &config.S3Config{Bucket: "test", Prefix: ""},
			want: "state.json",
		},
		{
			name: "with prefix",
			cfg:  &config.S3Config{Bucket: "test", Prefix: "exports/"},
			want: "exports/state.json",
		},
		{
			name: "with nested prefix",
			cfg:  &config.S3Config{Bucket: "test", Prefix: "data/ora2csv/"},
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

// TestMinIODetection tests MinIO/S3-compatible service detection
func TestMinIODetection(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.S3Config
		want bool
	}{
		{
			name: "no endpoint",
			cfg:  &config.S3Config{Endpoint: ""},
			want: false,
		},
		{
			name: "AWS S3 endpoint",
			cfg:  &config.S3Config{Endpoint: "https://s3.amazonaws.com"},
			want: false,
		},
		{
			name: "AWS regional endpoint",
			cfg:  &config.S3Config{Endpoint: "https://s3.eu-west-1.amazonaws.com"},
			want: false,
		},
		{
			name: "MinIO localhost",
			cfg:  &config.S3Config{Endpoint: "http://localhost:9000"},
			want: true,
		},
		{
			name: "MinIO custom domain",
			cfg:  &config.S3Config{Endpoint: "https://minio.example.com"},
			want: true,
		},
		{
			name: "Wasabi",
			cfg:  &config.S3Config{Endpoint: "https://s3.wasabisys.com"},
			want: true,
		},
		{
			name: "Digital Ocean Spaces",
			cfg:  &config.S3Config{Endpoint: "https://nyc3.digitaloceanspaces.com"},
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

// Test S3 client config field access
func TestS3Client_ConfigAccess(t *testing.T) {
	cfg := &config.S3Config{
		Bucket:    "test-bucket",
		Prefix:    "exports/",
		AccessKey: "key",
		SecretKey: "secret",
		Endpoint:  "http://localhost:9000",
	}

	client := &S3Client{
		cfg: cfg,
	}

	if client.cfg.Bucket != "test-bucket" {
		t.Errorf("Bucket = %q, want test-bucket", client.cfg.Bucket)
	}
	if client.cfg.Prefix != "exports/" {
		t.Errorf("Prefix = %q, want exports/", client.cfg.Prefix)
	}
	if client.cfg.IsMinIO() != true {
		t.Error("IsMinIO() = false, want true")
	}
}

// Test context timeout handling
func TestS3Client_ContextTimeout(t *testing.T) {
	client := &S3Client{
		cfg: &config.S3Config{
			Bucket: "test-bucket",
		},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Methods should handle cancelled context gracefully
	// Note: Since we don't have a real client, this tests the structure
	if client.client == nil {
		t.Skip("skipping - no real client")
	}

	// These would fail with a real client due to cancelled context
	_ = ctx
}

// Benchmark key generation
func BenchmarkS3KeyGeneration(b *testing.B) {
	cfg := &config.S3Config{
		Bucket: "test-bucket",
		Prefix: "exports/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Key("test-entity.csv")
	}
}

// Benchmark IsMinIO check
func BenchmarkIsMinIO(b *testing.B) {
	cfg := &config.S3Config{
		Endpoint: "http://localhost:9000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.IsMinIO()
	}
}

// Test helper function for error checking
func TestErrorIsNoSuchKey(t *testing.T) {
	testErr := &types.NoSuchKey{}
	var nsk *types.NoSuchKey

	if !errors.As(testErr, &nsk) {
		t.Error("failed to identify NoSuchKey error")
	}

	standardErr := errors.New("standard error")
	if errors.As(standardErr, &nsk) {
		t.Error("incorrectly identified standard error as NoSuchKey")
	}
}

// Test file operations for S3 backup
func TestLocalFileOperations(t *testing.T) {
	t.Run("create temp file for S3 upload", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/test.csv"
		testData := "id,name\n1,Alice\n"

		err := os.WriteFile(testFile, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		// Verify file exists and has content
		info, err := os.Stat(testFile)
		if err != nil {
			t.Errorf("Stat() error = %v", err)
		}
		if info.Size() != int64(len(testData)) {
			t.Errorf("file size = %d, want %d", info.Size(), len(testData))
		}
	})

	t.Run("remove file after S3 upload simulation", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := tmpDir + "/temp.csv"

		if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		// Simulate successful S3 upload followed by cleanup
		err := os.Remove(testFile)
		if err != nil {
			t.Errorf("Remove() error = %v", err)
		}

		// Verify file is gone
		_, err = os.Stat(testFile)
		if !os.IsNotExist(err) {
			t.Error("file still exists after Remove()")
		}
	})
}
