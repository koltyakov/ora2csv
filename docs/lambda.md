# AWS Lambda Deployment Guide

ora2csv can be deployed as an AWS Lambda function for scheduled, serverless Oracle-to-CSV exports.

## Overview

Running ora2csv in Lambda provides:
- **Scheduled execution** via EventBridge (cron-like scheduling)
- **No server management** - AWS handles infrastructure
- **Automatic scaling** for concurrent exports
- **Pay-per-use** pricing
- **Built-in monitoring** with CloudWatch

## Building the Lambda Binary

### For x86_64 (default)

```bash
# Build for Lambda (Linux x86_64)
GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd/ora2csv
zip function.zip bootstrap
```

### For ARM64 (Graviton - cost-optimized)

```bash
GOOS=linux GOARCH=arm64 go build -o bootstrap ./cmd/ora2csv
zip function.zip bootstrap
```

Note: ARM64 Lambda functions typically cost ~20% less than x86_64.

## Creating the Lambda Function

### Using AWS CLI

```bash
aws lambda create-function \
  --function-name ora2csv-export \
  --runtime provided.al2023 \
  --handler bootstrap \
  --zip-file fileb://function.zip \
  --role arn:aws:iam::ACCOUNT_ID:role/LambdaExecutionRole \
  --timeout 900 \
  --memory-size 1024 \
  --environment Variables={
    AWS_REGION=us-east-1,
    ORA2CSV_DB_PASSWORD=secret,
    ORA2CSV_S3_BUCKET=my-export-bucket,
    ORA2CSV_S3_PREFIX=production/,
    ORA2CSV_DB_HOST=oracle-prod.example.com,
    ORA2CSV_DB_USER=exporter,
    ORA2CSV_SQL_DIR=/var/task/sql
  }
```

### Key Configuration Parameters

| Parameter | Recommended Value | Description |
|-----------|-------------------|-------------|
| `timeout` | 900 (15 min) | Max execution time |
| `memory-size` | 1024-2048 MB | More memory = faster network |
| `runtime` | provided.al2023 | Custom runtime for Go binaries |
| `handler` | bootstrap | Lambda invokes this file |

## IAM Role Requirements

The Lambda execution role needs these permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-export-bucket",
        "arn:aws:s3:::my-export-bucket/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:DescribeNetworkInterfaces"
      ],
      "Resource": "*"
    }
  ]
}
```

### Creating the IAM Role

```bash
# Create trust policy
cat > trust-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "Service": "lambda.amazonaws.com"
    },
    "Action": "sts:AssumeRole"
  }]
}
EOF

# Create role
aws iam create-role \
  --role-name LambdaExecutionRole \
  --assume-role-policy-document file://trust-policy.json

# Attach basic Lambda execution policy
aws iam attach-role-policy \
  --role-name LambdaExecutionRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Attach S3 policy (replace with your bucket)
aws iam put-role-policy \
  --role-name LambdaExecutionRole \
  --policy-name S3Access \
  --policy-document file://s3-policy.json
```

## VPC Configuration (for VPC-protected databases)

If your Oracle database is in a VPC, configure Lambda with VPC access:

```bash
aws lambda update-function-configuration \
  --function-name ora2csv-export \
  --vpc-config SubnetIds=subnet-123456,subnet-789012,SecurityGroupIds=sg-123456
```

### VPC Considerations

- Lambda needs ENIs (Elastic Network Interfaces) in your VPC
- Ensure subnets have routes to the database
- Security group must allow outbound traffic to Oracle port (1521)
- Lambda may need VPC endpoint for S3 if S3 access is required

## Packaging SQL Files

### Option 1: Package with Lambda

```bash
# Create a deployment package
mkdir -p lambda-deployment/sql
cp bootstrap lambda-deployment/
cp sql/*.sql lambda-deployment/sql/
cd lambda-deployment
zip -r ../function.zip .

# Upload to Lambda
aws lambda update-function-code \
  --function-name ora2csv-export \
  --zip-file fileb://../function.zip
```

### Option 2: Store in S3

```bash
# Upload SQL files to S3
aws s3 sync sql/ s3://ora2csv-config/sql/

# Update Lambda environment to use S3 for SQL files
aws lambda update-function-configuration \
  --function-name ora2csv-export \
  --environment Variables={
    ORA2CSV_SQL_S3_BUCKET=ora2csv-config,
    ORA2CSV_SQL_S3_PREFIX=sql/
  }
```

## Scheduling with EventBridge

### Schedule Expressions

| Expression | Description |
|------------|-------------|
| `rate(1 hour)` | Every hour |
| `rate(30 minutes)` | Every 30 minutes |
| `cron(0 6 * * ? *)` | Daily at 6 AM UTC |
| `cron(0 */4 * * ? *)` | Every 4 hours |
| `cron(0 0 ? * MON-FRI *)` | Weekdays at midnight UTC |

### Creating Scheduled Rules

```bash
# Run every hour
aws events put-rule \
  --name ora2csv-hourly \
  --schedule-expression "rate(1 hour)"

# Run daily at 6 AM UTC
aws events put-rule \
  --name ora2csv-daily \
  --schedule-expression "cron(0 6 * * ? *)"

# Add Lambda as target
aws events put-targets \
  --rule ora2csv-hourly \
  --targets Id=1,Arn=arn:aws:lambda:us-east-1:ACCOUNT_ID:function:ora2csv-export

# Grant EventBridge permission to invoke Lambda
aws lambda add-permission \
  --function-name ora2csv-export \
  --statement-id ora2csv-hourly \
  --action lambda:InvokeFunction \
  --principal events.amazonaws.com \
  --source-arn arn:aws:events:us-east-1:ACCOUNT_ID:rule/ora2csv-hourly
```

## Monitoring

### CloudWatch Alarms

```bash
# Alert on Lambda errors
aws cloudwatch put-metric-alarm \
  --alarm-name ora2csv-errors \
  --alarm-description "Alert on ora2csv Lambda errors" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --threshold 1 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=FunctionName,Value=ora2csv-export

# Alert on long-running exports
aws cloudwatch put-metric-alarm \
  --alarm-name ora2csv-duration \
  --alarm-description "Alert on ora2csv Lambda duration" \
  --metric-name Duration \
  --namespace AWS/Lambda \
  --statistic Average \
  --period 300 \
  --threshold 800 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=FunctionName,Value=ora2csv-export

# Alert on throttles
aws cloudwatch put-metric-alarm \
  --alarm-name ora2csv-throttles \
  --alarm-description "Alert on ora2csv Lambda throttles" \
  --metric-name Throttles \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --threshold 1 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=FunctionName,Value=ora2csv-export
```

### Viewing Logs

```bash
# Get latest logs
aws logs tail /aws/lambda/ora2csv-export --follow

# Get logs for specific time range
aws logs filter-log-events \
  --log-group-name /aws/lambda/ora2csv-export \
  --start-time 1640000000000
```

## Terraform Deployment

Complete Terraform configuration for ora2csv Lambda:

```hcl
resource "aws_lambda_function" "ora2csv" {
  function_name = "ora2csv-export"
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  timeout       = 900
  memory_size   = 1024

  filename         = "function.zip"
  source_code_hash = filebase64sha256("function.zip")

  environment {
    variables = {
      AWS_REGION           = "us-east-1"
      ORA2CSV_DB_PASSWORD  = var.db_password
      ORA2CSV_S3_BUCKET    = "my-export-bucket"
      ORA2CSV_S3_PREFIX    = "production/"
      ORA2CSV_DB_HOST      = "oracle-prod.example.com"
      ORA2CSV_DB_USER      = "exporter"
      ORA2CSV_SQL_DIR      = "/var/task/sql"
    }
  }

  vpc_config {
    subnet_ids         = var.private_subnet_ids
    security_group_ids = [var.lambda_security_group_id]
  }
}

resource "aws_cloudwatch_log_group" "ora2csv" {
  name              = "/aws/lambda/ora2csv-export"
  retention_in_days = 30
}

resource "aws_cloudwatch_event_rule" "hourly" {
  name                = "ora2csv-hourly"
  schedule_expression = "rate(1 hour)"
}

resource "aws_cloudwatch_event_target" "ora2csv" {
  rule      = aws_cloudwatch_event_rule.hourly.name
  target_id = "ora2csv"
  arn       = aws_lambda_function.ora2csv.arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ora2csv.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.hourly.arn
}

# CloudWatch alarms
resource "aws_cloudwatch_metric_alarm" "errors" {
  alarm_name          = "ora2csv-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Sum"
  threshold           = "1"
  alarm_description   = "Alert on ora2csv Lambda errors"

  dimensions {
    name = "FunctionName"
    value = aws_lambda_function.ora2csv.function_name
  }
}

resource "aws_cloudwatch_metric_alarm" "duration" {
  alarm_name          = "ora2csv-duration"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Duration"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Average"
  threshold           = "800"
  alarm_description   = "Alert on ora2csv Lambda duration"

  dimensions {
    name = "FunctionName"
    value = aws_lambda_function.ora2csv.function_name
  }
}
```

### Terraform Variables

```hcl
variable "db_password" {
  type        = string
  description = "Oracle database password"
  sensitive   = true
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for Lambda VPC access"
}

variable "lambda_security_group_id" {
  type        = string
  description = "Security group ID for Lambda"
}
```

## Troubleshooting

### Lambda Times Out

- Increase timeout: `--timeout 1800` (30 minutes max)
- Increase memory: `--memory-size 2048` (more memory = faster CPU)
- Check database query performance

### VPC Connectivity Issues

```bash
# Check Lambda ENIs
aws ec2 describe-network-interfaces \
  --filters "Name=requester-id,Values=lambda.amazonaws.com"

# Test connectivity from Lambda (add to your code temporarily)
# Check security group allows outbound to database
```

### Credentials Error

Ensure IAM role has proper permissions:
- S3 write access for destination bucket
- VPC permissions for ENI creation (if using VPC)
- CloudWatch Logs permissions

### SQL Files Not Found

- Verify SQL files are in the zip package
- Check `ORA2CSV_SQL_DIR` environment variable
- For S3 approach, verify Lambda can access the config bucket

## Best Practices

1. **Use separate Lambda functions** for different environments (dev/staging/prod)
2. **Set appropriate memory** - 1024-2048 MB is usually optimal
3. **Configure dead-letter queue** (DLQ) for failed invocations
4. **Use X-Ray tracing** for performance analysis
5. **Package SQL files** with Lambda for faster cold starts
6. **Set CloudWatch log retention** to control costs
7. **Use S3 lifecycle policies** to automatically delete old exports
8. **Monitor Lambda cold starts** with CloudWatch metrics
