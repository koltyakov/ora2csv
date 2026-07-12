# AWS Lambda Support Status

The current ora2csv binary is a process-oriented Cobra CLI. It is not an AWS Lambda handler or custom runtime and cannot be deployed by renaming the binary to `bootstrap`.

## Recommended AWS Runtimes

For scheduled exports, run the CLI in an environment that supports normal process arguments and execution:

- ECS or Fargate scheduled tasks
- AWS Batch
- EC2 with systemd or cron
- A container-based CI/CD scheduler

These runtimes also avoid Lambda's 15-minute execution limit and provide more predictable temporary storage for large staged CSV files.

## Native Lambda Requirements

Native Lambda support would require a dedicated entry point that:

1. Uses `aws-lambda-go` or implements the Lambda Runtime API loop.
2. Invokes the exporter explicitly for each event.
3. Places state and staged CSV files under writable `/tmp` storage.
4. Packages SQL files or implements a supported remote SQL source.
5. Enforces single concurrency for each state object until distributed locking and conditional state updates are implemented.
6. Fits query and upload work within Lambda's 900-second maximum duration.

The environment variables `ORA2CSV_SQL_S3_BUCKET` and `ORA2CSV_SQL_S3_PREFIX` are not supported by the current CLI.
