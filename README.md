# ora2csv

Oracle to CSV exporter with state management and incremental sync support. A lightweight, cloud-friendly CLI tool written in Go that streams data directly from Oracle to CSV without storing entire exports in memory.

## Features

- **Streaming Export**: Direct Oracle-to-CSV streaming - no full dataset in memory
- **Incremental Sync**: State management tracks last run time per entity
- **S3 Storage Support**: Stream directly to Amazon S3 or S3-compatible services
- **Pure Go Oracle Driver**: No Oracle client installation required (uses `go-ora/v2`)
- **Single Binary**: Cloud-friendly deployment with no external dependencies
- **RFC 4180 CSV**: Proper CSV escaping and formatting
- **Entity-based Processing**: Process multiple entities from a single state file
- **Error Resilience**: Continue processing even if individual entities fail

## Installation

### From Source

```bash
go install github.com/koltyakov/ora2csv/cmd/ora2csv@latest
```

### Using Make

```bash
git clone https://github.com/koltyakov/ora2csv.git
cd ora2csv
make install
```

### Download Binary

Download the latest release from [Releases](https://github.com/koltyakov/ora2csv/releases).

## Quick Start

1. **Set database connection** via environment variables or flags:

   ```bash
   export ORA2CSV_DB_PASSWORD=your_password
   export ORA2CSV_DB_HOST=your_db_host
   export ORA2CSV_DB_USER=your_db_user
   ```

2. **Create state.json** (see Configuration below):

   ```json
   [
     {
       "entity": "crm.products",
       "lastRunTime": "2025-01-14T00:00:00",
       "active": true
     }
   ]
   ```

3. **Create SQL file** for each entity in `sql/` directory:

   ```sql
   -- sql/crm.products.sql
   SELECT
     id,
     name,
     sku,
     TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
   FROM crm.products
   WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
     AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
   ORDER BY updated ASC
   ```

4. **Run export**:
   ```bash
   ora2csv export
   ```

## Configuration

### Environment Variables

| Variable                | Description           | Default        |
| ----------------------- | --------------------- | -------------- |
| `ORA2CSV_DB_PASSWORD`   | Database password     | _required_     |
| `ORA2CSV_DB_HOST`       | Database host         | `dbserver`     |
| `ORA2CSV_DB_PORT`       | Database port         | `1521`         |
| `ORA2CSV_DB_SERVICE`    | Database service name | `ORCL`         |
| `ORA2CSV_DB_USER`       | Database user         | `system`       |
| `ORA2CSV_STATE_FILE`    | Path to state.json    | `./state.json` |
| `ORA2CSV_SQL_DIR`       | Path to SQL files     | `./sql`        |
| `ORA2CSV_EXPORT_DIR`    | Path for output CSVs  | `./export`     |
| `ORA2CSV_S3_BUCKET`     | S3 bucket name        | empty          |
| `ORA2CSV_S3_PREFIX`     | S3 key prefix         | empty          |
| `ORA2CSV_S3_ENDPOINT`   | S3 endpoint URL       | empty          |
| `AWS_ACCESS_KEY_ID`     | AWS access key        | (AWS SDK)      |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key        | (AWS SDK)      |
| `AWS_REGION`            | AWS region            | (AWS SDK)      |

For detailed S3 configuration, see the [S3 Storage Guide](docs/s3-guide.md).

### Command Flags

```bash
ora2csv export [flags]

Flags:
  --db-host string          Database host (default "dbserver")
  --db-port int             Database port (default 1521)
  --db-service string       Database service name (default "ORCL")
  --db-user string          Database user (default "system")
  --state-file string       Path to state.json (default "./state.json")
  --sql-dir string          Path to SQL directory (default "./sql")
  --export-dir string       Path to export directory (default "./export")
  --days-back int           Default days to look back for first run (default 30)
  --connect-timeout duration Connection timeout (default 30s)
  --query-timeout duration  Query timeout (default 5m)
  --s3-bucket string        S3 bucket name (enables S3 storage)
  --s3-prefix string        S3 key prefix
  --s3-endpoint string      S3 endpoint URL for S3-compatible services
  --s3-access-key string    S3 access key (for S3-compatible services)
  --s3-secret-key string    S3 secret key (for S3-compatible services)
  --s3-session-token string S3 session token (for S3-compatible services)
  --dry-run                Validate without executing
  --verbose                Enable verbose logging
```

### S3 Storage

For S3 configuration, examples, and S3-compatible service setup, see the [S3 Storage Guide](docs/s3-guide.md).

### State File Format

`state.json` defines entities to export:

```json
[
  {
    "entity": "crm.products",
    "lastRunTime": "2025-01-14T00:00:00",
    "active": true
  },
  {
    "entity": "crm.orders",
    "lastRunTime": "2025-01-14T10:00:00",
    "active": true
  },
  {
    "entity": "archive.entity",
    "lastRunTime": "2025-01-14T00:00:00",
    "active": false
  }
]
```

- **entity**: Name of the entity (must match `sql/<entity>.sql` filename)
- **lastRunTime**: ISO 8601 timestamp of last successful export
- **active**: Set to `false` to skip processing

## Commands

### export

Run data export for all active entities:

```bash
ora2csv export
```

Dry run (validate only):

```bash
ora2csv export --dry-run
```

### validate

Validate configuration and SQL files:

```bash
ora2csv validate
```

With database connection test:

```bash
ora2csv validate --test-connection
```

## How It Works

1. **Load State**: Read `state.json` to get entities and their last run times
2. **Calculate Time Range**:
   - `:tillDate` = Current timestamp
   - `:startDate` = Previous `lastRunTime` (or 30 days ago for first run)
3. **Execute SQL**: For each active entity, execute `sql/<entity>.sql` with bind variables
4. **Stream to CSV**: Write results directly to `<entity>__<startDate>.csv`
5. **Update State**: On success, update `lastRunTime` to current timestamp
6. **Continue**: Process remaining entities even if some fail

## SQL File Guidelines

SQL files should:

1. Use bind variables `:startDate` and `:tillDate` for time filtering:

   ```sql
   WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
     AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
   ```

2. Order by timestamp for consistent streaming:

   ```sql
   ORDER BY updated ASC
   ```

3. Format timestamps as ISO 8601:

   ```sql
   TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
   ```

4. **No trailing semicolon** - Oracle's programmatic interface does not accept SQL statements terminated with semicolons

## Output

### CSV Files

- Location: `export/<entity>__<startDate>.csv`
- Format: RFC 4180 compliant
- NULL values: Empty strings
- Encoding: UTF-8

### Exit Codes

- `0` - All entities successful
- `1` - Configuration/initialization error
- `2` - One or more entities failed (but others succeeded)

### Example Output

```
[2025-01-14 16:30:00] Starting ora2csv v1.0.0 (built: 2025-01-14T16:00:00Z)
[2025-01-14 16:30:00] Loaded state file: ./state.json (3 entities, 2 active)
[2025-01-14 16:30:00] Connecting to database: system@dbserver:1521/ORCL
[2025-01-14 16:30:01] Database connection established
[2025-01-14 16:30:01] Using till date for all entities: 2025-01-14T16:30:01
[2025-01-14 16:30:01] [crm.products] Processing entity: crm.products
[2025-01-14 16:30:01] [crm.products] Start date: 2025-01-14T00:00:00
[2025-01-14 16:30:02] [crm.products] Exported 1234 rows to: export/crm.products__2025-01-14T00-00-00.csv
==================================================
[2025-01-14 16:30:02] Export completed successfully
[2025-01-14 16:30:02] Total duration: 0m 1s
[2025-01-14 16:30:02] Successfully processed: 2
[2025-01-14 16:30:02] Skipped (inactive): 1
==================================================
```

## Use Cases

### Data Warehouse Ingestion

ora2csv is commonly used for periodic incremental data export to data warehouses. See [Data Warehouse Ingestion Guide](docs/datawarehouse-use-case.md) for:

- Architecture patterns (Oracle → ora2csv → S3 → Snowflake/BigQuery)
- Incremental sync setup
- SQL file design for merge operations
- Integration examples with dbt, Airflow, and other orchestration tools

## Development

### Prerequisites

- Go 1.21 or later

### E2E Testing

For local testing with an Oracle database, see the [e2e/](e2e/) directory. It includes:

- Docker Compose setup with Oracle XE
- Test data generation scripts
- Example SQL queries and state files

### Building

```bash
make build        # Build for current platform
make build-all    # Build for all platforms
make test         # Run tests
make lint         # Run linter
```

## License

MIT License
