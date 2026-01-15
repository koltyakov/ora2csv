# Data Warehouse Ingestion with ora2csv

This document describes how to use ora2csv for periodic data export from Oracle to data warehouse solutions.

## Overview

ora2csv is designed for **incremental data synchronization** from Oracle databases to data warehouse staging areas. It streams changed records to CSV files that can be consumed by ETL/ELT pipelines.

## Architecture

```
┌─────────────────┐      ┌───────────────┐      ┌─────────────────┐
│  Oracle ERP     │ ───> │    ora2csv    │ ───> │  Object Storage │
│ (Source System) │      │ (Incremental) │      │  (S3/GCS/Azure) │
└─────────────────┘      └───────────────┘      └─────────────────┘
                                                         │
                                                         v
┌─────────────────┐      ┌───────────────┐      ┌─────────────────┐
│  Data Warehouse │ <─── │  ETL/ELT      │ <─── │   Staging Area  │
│   (Snowflake/   │      │ (dbt/Airflow) │      │   (.csv files)  │
│   BigQuery/     │      └───────────────┘      └─────────────────┘
│   Redshift/     │
│   Databricks)   │
└─────────────────┘
```

## Key Features for Data Warehousing

### 1. Incremental Sync

Only exports changed records using timestamp-based filtering:

```sql
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
```

### 2. State Management

`state.json` tracks last run time per entity, enabling reliable incremental exports:

```json
[
  {
    "entity": "oracle.customers",
    "lastRunTime": "2025-01-14T10:00:00",
    "active": true
  },
  {
    "entity": "oracle.orders",
    "lastRunTime": "2025-01-14T10:00:00",
    "active": true
  }
]
```

### 3. Streaming Design

Direct Oracle-to-CSV streaming avoids memory spikes for large exports. The `--days-back` parameter controls initial export window.

## Typical Workflow

### Hourly Incremental Export

```bash
#!/bin/bash
# hourly-export.sh

DATEstamp=$(date +%Y%m%d_%H%M)
STAGING_DIR="/data/staging/${DATEstamp}"

# Export changed records
ora2csv export \
  --state-file /data/oracle_state.json \
  --sql-dir /app/sql \
  --export-dir "$STAGING_DIR" \
  --db-host "$ORACLE_HOST" \
  --db-service "$ORACLE_SERVICE" \
  --db-user "$ORACLE_USER"

# Upload to cloud storage
aws s3 sync "$STAGING_DIR" s3://my-dwh-staging/oracle/

# Trigger dbt/ELT (optional)
curl -X POST https://my-pipeline/api/trigger
```

### Cron Schedule

```crontab
# Hourly incremental export
0 * * * * /app/hourly-export.sh >> /var/log/ora2csv.log 2>&1
```

## Data Warehouse Merge Pattern

The exported CSVs are typically loaded into staging tables and merged into target tables.

### Snowflake Example

```sql
-- Load CSV to staging
COPY INTO staging.customers (id, name, email, updated)
FROM @dwh_staging/oracle/customers__.csv
FILE_FORMAT = (TYPE = CSV FIELD_DELIMITER = ',');

-- Merge into target
MERGE INTO dw.customers target
USING staging.customers src
ON target.id = src.id
WHEN MATCHED AND src.updated > target.updated THEN
  UPDATE SET
    name = src.name,
    email = src.email,
    updated = src.updated
WHEN NOT MATCHED THEN
  INSERT (id, name, email, updated)
  VALUES (src.id, src.name, src.email, src.updated);
```

### BigQuery Example

```sql
-- Load CSV to staging
LOAD DATA INTO dw.staging_customers
FROM FILES (
  format = 'CSV',
  uris = ['gs://dwh-staging/oracle/customers__*.csv']
);

-- Merge into target (using MERGE statement or INSERT/UPDATE)
MERGE dw.customers T
USING dw.staging_customers S
ON T.id = S.id
WHEN MATCHED AND S.updated > T.updated THEN
  UPDATE SET name = S.name, email = S.email
WHEN NOT MATCHED THEN
  INSERT (id, name, email) VALUES (S.id, S.name, S.email);
```

## Best Practices

### 1. SQL File Design

Include `updated` timestamp for merge logic:

```sql
-- sql/oracle.customers.sql
SELECT
  id,
  first_name,
  last_name,
  email,
  TO_CHAR(created, 'YYYY-MM-DD"T"HH24:MI:SS') as created,
  TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM oracle.customers
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY updated ASC
```

### 2. File Naming

The default `entity__timestamp.csv` format supports:
- Easy identification of export windows
- Historical comparison
- Idempotent reprocessing

### 3. Error Handling

Combine with orchestration tools for robust pipelines:

```yaml
# Airflow example
export_oracle:
  operator: BashOperator
  bash_command: |
    ora2csv export \
      --state-file /data/oracle_state.json \
      --sql-dir /app/sql \
      --export-dir /data/staging/{{ ds_nodash }}
  on_failure_callback: notify_alert
  retries: 3
```

### 4. Monitoring

Track exports for observability:

```bash
# Log export metrics
ora2csv export --verbose | tee -a /var/log/ora2csv.log

# Parse for metrics
tail -100 /var/log/ora2csv.log | grep "rows"
```

## Example: Full Pipeline Setup

### Directory Structure

```
/app/
├── sql/
│   ├── oracle.customers.sql
│   ├── oracle.orders.sql
│   └── oracle.products.sql
├── state/
│   └── state.json
├── scripts/
│   ├── hourly-export.sh
│   └── upload-to-s3.sh
└── export/
    └── (generated CSV files)
```

### Initial Load

```bash
# First run: export last 30 days
ora2csv export \
  --days-back 30 \
  --sql-dir /app/sql \
  --state-file /app/state/state.json \
  --export-dir /app/export
```

### Incremental Updates

```bash
# Subsequent runs: only changed records
ora2csv export \
  --sql-dir /app/sql \
  --state-file /app/state/state.json \
  --export-dir /app/export
```

## Integration Examples

### With dbt

```python
# models/staging/oracle_customers.sql
{{ config(
    materialized='incremental',
    unique_key='id',
    incremental_strategy='merge'
) }}

SELECT * FROM {{ source('staging', 'oracle_customers') }}
{% if is_incremental() %}
WHERE updated > (SELECT MAX(updated) FROM {{ this }})
{% endif %}
```

### With Airflow

```python
from airflow import DAG
from airflow.operators.bash import BashOperator

dag = DAG('oracle_to_dwh', schedule_interval='@hourly')

export_task = BashOperator(
    task_id='export_oracle',
    bash_command='ora2csv export --state-file /data/state.json',
    dag=dag
)

upload_task = BashOperator(
    task_id='upload_s3',
    bash_command='aws s3 sync /data/export s3://dwh-staging',
    dag=dag
)

export_task >> upload_task
```

## Troubleshooting

### Missing Records

Ensure SQL filters by `updated` timestamp correctly:

```sql
-- Correct: includes records updated exactly at startDate
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')

-- Incorrect: excludes records at boundary
WHERE updated > TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
```

### Duplicate Records

Order by timestamp to ensure deterministic export:

```sql
ORDER BY updated ASC
```

### Large Exports

Use `--days-back` to limit initial export window:

```bash
# Export only last 7 days on first run
ora2csv export --days-back 7
```
