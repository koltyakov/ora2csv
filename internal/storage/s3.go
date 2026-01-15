package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/koltyakov/ora2csv/internal/config"
)

// S3Client wraps AWS S3 operations for ora2csv
type S3Client struct {
	client   *s3.Client
	uploader *manager.Uploader
	cfg      *config.S3Config
}

// NewS3Client creates a new S3 client from configuration
func NewS3Client(cfg *config.S3Config) (*S3Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	// Build AWS configuration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var awsCfg aws.Config
	var err error

	// Custom endpoint resolver for S3-compatible services (MinIO, etc.)
	if cfg.Endpoint != "" {
		// Use static credentials when endpoint is custom
		// Region is required by AWS SDK but not used for custom endpoints
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion("us-east-1"),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey,
				cfg.SecretKey,
				cfg.SessionToken,
			)),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		// Create S3 client with custom endpoint
		client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})

		// Configure multipart upload with fixed 5MB part size
		uploader := manager.NewUploader(client, func(u *manager.Uploader) {
			u.PartSize = 5 * 1024 * 1024 // 5MB
			u.Concurrency = 5            // Default concurrency for multipart upload
		})

		return &S3Client{
			client:   client,
			uploader: uploader,
			cfg:      cfg,
		}, nil
	}

	// Use default AWS credential chain for AWS S3
	// Region is loaded from AWS_REGION env var or AWS config
	awsCfg, err = awsconfig.LoadDefaultConfig(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// Disable SSL verification for local development if needed
		// o.UsePathStyle = true // Useful for MinIO
	})

	// Configure multipart upload with fixed 5MB part size
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 5 * 1024 * 1024 // 5MB
		u.Concurrency = 5            // Default concurrency for multipart upload
	})

	return &S3Client{
		client:   client,
		uploader: uploader,
		cfg:      cfg,
	}, nil
}

// UploadFile uploads a local file to S3
func (s *S3Client) UploadFile(ctx context.Context, key, path string) error {
	// For streaming, we should use UploadStream with a file reader
	// This method is a convenience wrapper
	return fmt.Errorf("use UploadStream for file uploads")
}

// UploadStream uploads data from an io.Reader to S3 using multipart upload
func (s *S3Client) UploadStream(ctx context.Context, key string, r io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
		Body:   r,
	}

	_, err := s.uploader.Upload(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload to S3 (key=%s): %w", key, err)
	}

	return nil
}

// DownloadStream downloads an object from S3 as an io.ReadCloser
func (s *S3Client) DownloadStream(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}

	output, err := s.client.GetObject(ctx, input)
	if err != nil {
		var nsk *types.NoSuchKey
		if ok := errors.As(err, &nsk); ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to download from S3 (key=%s): %w", key, err)
	}

	return output.Body, nil
}

// DownloadFile downloads an object from S3 and writes it to a local file
func (s *S3Client) DownloadFile(ctx context.Context, key, path string) error {
	reader, err := s.DownloadStream(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Create the file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	// Copy the data
	_, err = io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// Exists checks if a key exists in S3
func (s *S3Client) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		var nsk *types.NoSuchKey
		if ok := errors.As(err, &nsk); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check S3 object existence (key=%s): %w", key, err)
	}

	return true, nil
}

// Delete deletes an object from S3
func (s *S3Client) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete from S3 (key=%s): %w", key, err)
	}

	return nil
}

// ListPrefix lists all objects with a given prefix
func (s *S3Client) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.Bucket),
		Prefix: aws.String(prefix),
	}

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, input, func(o *s3.ListObjectsV2PaginatorOptions) {
		o.Limit = 1000
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects (prefix=%s): %w", prefix, err)
		}

		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}

	return keys, nil
}

// UploadBytes uploads a byte slice to S3
func (s *S3Client) UploadBytes(ctx context.Context, key string, data []byte) error {
	return s.UploadStream(ctx, key, bytes.NewReader(data))
}

// DownloadBytes downloads an object from S3 as a byte slice
func (s *S3Client) DownloadBytes(ctx context.Context, key string) ([]byte, error) {
	reader, err := s.DownloadStream(ctx, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// CheckConnection verifies S3 connectivity and PutObject permissions
// It uploads a small test object and then deletes it
func (s *S3Client) CheckConnection(ctx context.Context) error {
	testKey := ".ora2csv-connectivity-test"

	// Try to upload a small object (tests PutObject permission)
	putInput := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(testKey),
		Body:   bytes.NewReader([]byte("connectivity check")),
	}

	_, err := s.client.PutObject(ctx, putInput)
	if err != nil {
		return fmt.Errorf("S3 connection check failed: %w", err)
	}

	// Clean up the test object
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(testKey),
	}

	_, err = s.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		// Log warning but don't fail - the upload succeeded
		return fmt.Errorf("S3 connection check succeeded but cleanup failed: %w", err)
	}

	return nil
}
