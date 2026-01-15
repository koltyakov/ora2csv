# S3 Storage Guide

ora2csv supports streaming CSV exports directly to Amazon S3 or S3-compatible storage services (MinIO, Wasabi, etc.). When S3 is enabled, files are uploaded via multipart upload and the state file is synchronized with S3.

## Features

- **Multipart Upload**: Efficient streaming of large files (5MB part size)
- **State Synchronization**: State file is fetched from and uploaded to S3
- **S3-Compatible Services**: Support for MinIO, Wasabi, and other S3-like services
- **Standard AWS Credentials**: Uses AWS SDK credential chain (supports `aws-vault`, AWS CLI profiles)
- **Automatic Region Detection**: Reads from `AWS_REGION` environment variable

## Quick Start

### AWS S3

1. **Set up credentials** using standard AWS environment variables:

   ```bash
   export AWS_ACCESS_KEY_ID=your_access_key
   export AWS_SECRET_ACCESS_KEY=your_secret_key
   export AWS_REGION=us-west-2
   ```

   Or use an AWS profile:

   ```bash
   export AWS_PROFILE=production
   ```

   Or use `aws-vault`:

   ```bash
   aws-vault exec production -- ora2csv export
   ```

2. **Run export with S3 bucket**:

   ```bash
   ora2csv export --s3-bucket=my-export-bucket
   ```

### MinIO / S3-Compatible Services

For S3-compatible services, use the `--s3-endpoint` flag with access keys:

```bash
ora2csv export \
  --s3-bucket=exports \
  --s3-endpoint=https://minio.example.com \
  --s3-access-key=minioadmin \
  --s3-secret-key=minioadmin
```

## Configuration

### Command Flags

| Flag                 | Description                                    | Default           |
| -------------------- | ---------------------------------------------- | ----------------- |
| `--s3-bucket`        | S3 bucket name                                 | _required for S3_ |
| `--s3-prefix`        | S3 key prefix (e.g., `exports/`)               | empty             |
| `--s3-endpoint`      | Custom endpoint URL for S3-compatible services | empty             |
| `--s3-access-key`    | Access key for S3-compatible services          | empty             |
| `--s3-secret-key`    | Secret key for S3-compatible services          | empty             |
| `--s3-session-token` | Session token for S3-compatible services       | empty             |

### Environment Variables

| Variable                | Description                  |
| ----------------------- | ---------------------------- |
| `ORA2CSV_S3_BUCKET`     | S3 bucket name               |
| `ORA2CSV_S3_PREFIX`     | S3 key prefix                |
| `ORA2CSV_S3_ENDPOINT`   | Custom endpoint URL          |
| `AWS_ACCESS_KEY_ID`     | AWS access key (standard)    |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key (standard)    |
| `AWS_SESSION_TOKEN`     | AWS session token (standard) |
| `AWS_REGION`            | AWS region (standard)        |
| `AWS_PROFILE`           | AWS profile name (standard)  |

## S3 File Layout

Files are organized in S3 by entity name, with each entity in its own folder:

```
s3://bucket-name/
└── [prefix/]                     # Optional prefix
    ├── state.json                # State file (synced)
    ├── entity1/
    │   ├── entity1__2025-01-14T00-00-00.csv
    │   └── entity1__2025-01-15T00-00-00.csv
    ├── entity2/
    │   └── entity2__2025-01-14T00-00-00.csv
    └── ...
```

This structure keeps all exports for the same entity together in one folder.

## State Synchronization

When S3 is enabled:

1. **On startup**: ora2csv tries to fetch `state.json` from S3 first

   - If found in S3: downloads and uses it
   - If not found: falls back to local `state.json`
   - If neither exists: starts with empty state

2. **After each entity**: updates `state.json` locally and uploads to S3

3. **On S3 upload failure**: local state is preserved, warning is logged

## Examples

### Basic S3 Export

```bash
export AWS_REGION=us-west-2
ora2csv export --s3-bucket=my-data-exports
```

### With Prefix

```bash
ora2csv export \
  --s3-bucket=my-data-exports \
  --s3-prefix=production/$(date +%Y%m%d)/
```

### Full Configuration Example

```bash
export AWS_REGION=eu-central-1
export ORA2CSV_DB_PASSWORD=secret

ora2csv export \
  --s3-bucket=company-exports \
  --s3-prefix=oracle/crm/ \
  --db-host=oracle-prod.internal \
  --db-user=exporter \
  --sql-dir=/opt/ora2csv/sql \
  --verbose
```

### MinIO with Docker

```bash
# Start MinIO
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  --name minio \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Run ora2csv with MinIO
ora2csv export \
  --s3-bucket=exports \
  --s3-endpoint=http://localhost:9000 \
  --s3-access-key=minioadmin \
  --s3-secret-key=minioadmin
```

### Using aws-vault

```bash
# AWS profile from ~/.aws/config
aws-vault exec prod-role -- ora2csv export --s3-bucket=prod-exports

# With MFA
aws-vault exec prod-role -- mfa=true ora2csv export --s3-bucket=prod-exports
```

### Using IAM Role (EC2/Lambda/ECS)

When running on AWS infrastructure with IAM role:

```bash
# No credentials needed - IAM role provides them
ora2csv export --s3-bucket=my-export-bucket
```

## S3-Compatible Services

### Wasabi

```bash
ora2csv export \
  --s3-bucket=exports \
  --s3-endpoint=https://s3.wasabisys.com \
  --s3-access-key=WASABI_ACCESS_KEY \
  --s3-secret-key=WASABI_SECRET_KEY
```

### DigitalOcean Spaces

```bash
ora2csv export \
  --s3-bucket=my-space \
  --s3-endpoint=https://nyc3.digitaloceanspaces.com \
  --s3-access-key=SPACES_KEY \
  --s3-secret-key=SPACES_SECRET
```

### Storj DCS

```bash
ora2csv export \
  --s3-bucket=my-bucket \
  --s3-endpoint=https://gateway.storjshare.io \
  --s3-access-key=STORJ_ACCESS_KEY \
  --s3-secret-key=STORJ_SECRET_KEY
```

## Error Handling

### S3 Upload Failure

If S3 upload fails, the local CSV file is kept as a fallback:

```
[2025-01-14 16:30:00] [entity] S3 upload failed: operation error S3: PutObject...
(local file kept at /path/to/export/entity__2025-01-14T00-00-00.csv)
```

The export process continues with other entities.

### State Upload Failure

If state upload to S3 fails, a warning is logged but the export continues:

```
Warning: failed to upload state to S3: ...
```

The local state file is preserved and will be used on the next run.

## Best Practices

### Security

1. **Use IAM roles** when running on AWS infrastructure (EC2, Lambda, ECS)
2. **Use aws-vault** for local development with MFA
3. **Never commit credentials** to version control
4. **Use bucket policies** instead of access keys when possible

### Performance

1. **Multipart upload** is automatic for files larger than 5MB
2. **Concurrency** of 5 is used for multipart uploads
3. **Network timeout** is set to 5 minutes per file upload

### Organization

1. **Separate buckets** for different environments (dev/staging/prod)
2. **Use prefixes** to organize exports by environment or system
3. **Enable S3 lifecycle policies** to automatically delete old exports

   Use AWS S3 Lifecycle rules to automatically expire old export files:

   ```bash
   # Create a lifecycle policy to delete exports older than 90 days
   aws s3api put-bucket-lifecycle-configuration \
     --bucket my-export-bucket \
     --lifecycle-configuration '{
       "Rules": [
         {
           "Id": "DeleteOldExports",
           "Status": "Enabled",
           "Filter": {"Prefix": "exports/"},
           "Expiration": {"Days": 90},
           "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 7}
         }
       ]
     }'
   ```

   For more granular control, use prefix-based rules:

   ```bash
   # Production: keep for 365 days
   # Staging: keep for 90 days
   # Dev: keep for 30 days
   aws s3api put-bucket-lifecycle-configuration \
     --bucket my-export-bucket \
     --lifecycle-configuration file://lifecycle.json
   ```

   Example `lifecycle.json`:

   ```json
   {
     "Rules": [
       {
         "Id": "DeleteDevExports",
         "Filter": {"Prefix": "dev/"},
         "Status": "Enabled",
         "Expiration": {"Days": 30}
       },
       {
         "Id": "DeleteStagingExports",
         "Filter": {"Prefix": "staging/"},
         "Status": "Enabled",
         "Expiration": {"Days": 90}
       },
       {
         "Id": "DeleteProdExports",
         "Filter": {"Prefix": "prod/"},
         "Status": "Enabled",
         "Expiration": {"Days": 365}
       }
     ]
   }
   ```

## Troubleshooting

### Connection Issues

Enable verbose logging to see S3 operations:

```bash
ora2csv export --s3-bucket=my-bucket --verbose
```

### Credential Issues

Test AWS credentials first:

```bash
aws s3 ls s3://my-bucket
```

Or with aws-vault:

```bash
aws-vault exec prod-role -- aws s3 ls s3://my-bucket
```

### MinIO SSL Issues

For self-signed certificates in development, you may need to disable SSL verification in your MinIO client configuration or use a proper certificate.

### Region Mismatch

Ensure `AWS_REGION` matches your bucket region:

```bash
export AWS_REGION=us-east-1
ora2csv export --s3-bucket=my-bucket
```
