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
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/koltyakov/ora2csv/internal/config"
)

var ErrCASConflict = errors.New("S3 compare-and-swap conflict")

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
	if err := cfg.Validate(); err != nil {
		return nil, err
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
			o.UsePathStyle = true
		})

		// Configure multipart upload with fixed 5MB part size
		uploader := newUploader(client, cfg)

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
	uploader := newUploader(client, cfg)

	return &S3Client{
		client:   client,
		uploader: uploader,
		cfg:      cfg,
	}, nil
}

func newUploader(client *s3.Client, cfg *config.S3Config) *manager.Uploader {
	return manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = cfg.PartSize
		u.Concurrency = cfg.Concurrency
	})
}

// UploadFile uploads a local file to S3
func (s *S3Client) UploadFile(ctx context.Context, key, path string) error {
	// For streaming, we should use UploadStream with a file reader
	// This method is a convenience wrapper
	return fmt.Errorf("use UploadStream for file uploads")
}

// UploadStream uploads data from an io.Reader to S3 using multipart upload
func (s *S3Client) UploadStream(ctx context.Context, key string, r io.Reader) error {
	uploadCtx, cancel := context.WithTimeout(ctx, s.cfg.UploadTimeout)
	defer cancel()
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
		Body:   r,
	}

	_, err := s.uploader.Upload(uploadCtx, input)
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
		if isNotFound(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to download from S3 (key=%s): %w", key, err)
	}

	return output.Body, nil
}

// DownloadFile downloads an object from S3 and writes it to a local file
func (s *S3Client) DownloadFile(ctx context.Context, key, path string) (retErr error) {
	reader, err := s.DownloadStream(ctx, key)
	if err != nil {
		return err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close S3 download stream: %w", err))
		}
	}()

	// Create the file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close file %s: %w", path, err))
		}
	}()

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
		if isNotFound(err) {
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

// UploadBytesCAS conditionally replaces a small object and returns its new ETag.
func (s *S3Client) UploadBytesCAS(ctx context.Context, key string, data []byte, expectedETag *string) (string, error) {
	uploadCtx, cancel := context.WithTimeout(ctx, s.cfg.UploadTimeout)
	defer cancel()
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}
	if expectedETag == nil {
		input.IfNoneMatch = aws.String("*")
	} else {
		input.IfMatch = expectedETag
	}
	output, err := s.client.PutObject(uploadCtx, input)
	if err != nil {
		if isCASConflict(err) {
			return "", fmt.Errorf("%w for key %s: %v", ErrCASConflict, key, err)
		}
		return "", fmt.Errorf("failed to conditionally upload to S3 (key=%s): %w", key, err)
	}
	if output.ETag == nil || *output.ETag == "" {
		return "", fmt.Errorf("conditional S3 upload returned no ETag for key %s", key)
	}
	return *output.ETag, nil
}

// DownloadBytes downloads an object from S3 as a byte slice
func (s *S3Client) DownloadBytes(ctx context.Context, key string) (data []byte, retErr error) {
	data, _, err := s.DownloadBytesVersion(ctx, key)
	return data, err
}

// DownloadBytesVersion downloads a small object and returns its opaque ETag.
// A missing object returns nil data, nil ETag, and no error.
func (s *S3Client) DownloadBytesVersion(ctx context.Context, key string) (data []byte, etag *string, retErr error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to download from S3 (key=%s): %w", key, err)
	}
	defer func() {
		if err := output.Body.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("failed to close S3 download stream: %w", err))
		}
	}()

	data, err = io.ReadAll(output.Body)
	if err != nil {
		return nil, nil, err
	}
	if output.ETag == nil || *output.ETag == "" {
		return nil, nil, fmt.Errorf("S3 download returned no ETag for key %s", key)
	}
	return data, output.ETag, nil
}

// CheckConnection verifies that the configured bucket is reachable.
func (s *S3Client) CheckConnection(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 bucket check failed: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound")
}

func isCASConflict(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		if code == "PreconditionFailed" || code == "ConditionalRequestConflict" {
			return true
		}
	}
	var responseErr *smithyhttp.ResponseError
	return errors.As(err, &responseErr) && (responseErr.HTTPStatusCode() == 409 || responseErr.HTTPStatusCode() == 412)
}
