# S3 Storage Guide

ora2csv supports staged CSV uploads to Amazon S3 or S3-compatible storage services (MinIO, Wasabi, etc.). Rows stream from Oracle to a local temporary file; after the CSV is complete, it is atomically published locally and uploaded to S3. The state file is synchronized with S3.

## Features

- **Multipart Upload**: Completed files are uploaded with 5MB parts when multipart upload applies
- **Transactional Publication**: Incomplete CSV files are not exposed under their final names
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
  --s3-secret-key=minioadmin \
  --s3-allow-insecure-endpoint
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
| `--s3-upload-timeout` | Maximum duration per S3 upload                 | `5m`              |
| `--s3-part-size`      | Multipart part size in bytes                   | `5242880`         |
| `--s3-concurrency`    | Multipart upload concurrency                   | `5`               |
| `--s3-allow-insecure-endpoint` | Allow a plaintext HTTP endpoint       | `false`           |

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
| `ORA2CSV_S3_ALLOW_INSECURE_ENDPOINT` | Permit plaintext HTTP custom endpoints |

### IAM Permissions

The bucket preflight is read-only. Grant `s3:ListBucket` on the bucket and `s3:GetObject`, `s3:PutObject`, and `s3:AbortMultipartUpload` on the configured object prefix. `s3:DeleteObject` is not required by ora2csv.

## S3 File Layout

Files are organized in S3 by entity name, with each entity in its own folder:

```
s3://bucket-name/
└── [prefix/]                     # Optional prefix
    ├── state.json                # State file (synced)
    ├── entity1/
    │   ├── entity1__2025-01-14T00-00-00__2025-01-15T00-00-00.csv
    │   └── entity1__2025-01-15T00-00-00__2025-01-16T00-00-00.csv
    ├── entity2/
    │   └── entity2__2025-01-14T00-00-00__2025-01-15T00-00-00.csv
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

3. **On state upload failure**: the entity is marked failed, later entities continue, and the remote state remains authoritative

S3 mode requires enough local disk for one completed entity export. A successful upload removes the local CSV; a failed upload retains it as a fallback.

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
  --s3-secret-key=minioadmin \
  --s3-allow-insecure-endpoint
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

### Running on AWS Lambda

The current binary is a process-oriented CLI, not a Lambda custom runtime. See [Lambda Support Status](lambda.md) for supported alternatives and the work required for native Lambda support.

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

If a CSV upload fails, that entity fails, its completed local CSV is retained, and later entities continue. The command exits with status 2 after processing the remaining entities:

```
[2026-01-14 16:30:00] [entity] S3 upload failed: operation error S3: PutObject...
Error: entity entity1 failed: S3 upload failed: ... (local file kept at /path/to/export/entity__2025-01-14T00-00-00__2025-01-15T00-00-00.csv)
```

The local CSV file is preserved as a fallback. Fix the S3 credentials or connectivity issue, then retry.

### State Upload Failure

If state upload to S3 fails, that entity is marked failed and later entities continue:

```
Error: failed to update state for entity1: failed to upload state to S3 (key=prefix/state.json): ...
```

The local copy is retained for diagnosis, but an existing S3 state object remains authoritative on the next run.

## Best Practices

### Security

1. **Use IAM roles** when running on AWS infrastructure (EC2 or ECS)
2. **Use aws-vault** for local development with MFA
3. **Never commit credentials** to version control
4. **Use bucket policies** instead of access keys when possible

### Performance

1. **Multipart upload** is automatic for files larger than the configured part size
2. **Concurrency** is configurable with `--s3-concurrency`
3. **Upload timeout** is configurable with `--s3-upload-timeout`
4. **Local capacity** must accommodate one completed entity CSV

### Concurrency

Local exporters sharing the same state path are serialized with an advisory lock. S3 state updates use ETag compare-and-swap, preventing a stale process from overwriting newer state. Separate hosts can still run duplicate exports before one receives a state conflict; distributed execution locking is not yet implemented.

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
