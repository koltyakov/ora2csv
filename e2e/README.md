# ora2csv E2E Testing

End-to-end testing environment for ora2csv with a local Oracle database.

## Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for building ora2csv)

**No Oracle Instant Client or Atlas required** - Schema is automatically created on container startup, and all scripts use `docker exec`.

## Quick Start

### 1. Start Oracle Database

```bash
cd e2e
docker compose up -d
```

Wait for Oracle to start (about 30-60 seconds):

```bash
docker compose logs -f oracle
```

Look for `DATABASE IS READY TO USE!` message.

### 2. Run Schema Migrations

The schema is automatically created when the container starts for the first time via `scripts/01-init-schema.sql`.

If you need to reinitialize the schema (e.g., after `docker compose down -v`):

```bash
# Restart with fresh volume
docker compose down -v
docker compose up -d
```

### 3. Generate Test Data

```bash
./scripts/generate-data.sh
```

This script uses `docker exec` to run sqlplus inside the container - no Oracle client needed on your host.

### 4. Build ora2csv

```bash
cd ..
make build
```

### 5. Run Export

```bash
cd e2e
./run-export.sh export --verbose
```

Or with direct flags:

```bash
export ORA2CSV_DB_PASSWORD=ora2csv_pass
../bin/ora2csv export \
  --db-host localhost \
  --db-service ORCL \
  --db-user ora2csv \
  --state-file ./state.json \
  --sql-dir ./sql \
  --export-dir ./export \
  --verbose
```

## Project Structure

```
e2e/
├── docker-compose.yml     # Oracle XE container definition
├── scripts/              # Initialization and data management
│   ├── 01-init-schema.sql    # Schema creation (runs on container start)
│   ├── generate-data.sh      # Generate initial test data
│   ├── add-new-data.sh       # Add new records for testing
│   └── modify-data.sh        # Update existing records
├── sql/                  # ora2csv query templates
│   ├── crm.products.sql
│   ├── crm.orders.sql
│   ├── crm.customers.sql
│   └── archive.entity.sql
├── state.json            # Initial state file
├── run-export.sh         # Helper to run ora2csv with test DB
└── export/               # Generated CSV files (created on first run)
```

## Testing Scenarios

### Scenario 1: Initial Export (All Data)

First export with `lastRunTime` set to past date captures all records:

```bash
./run-export.sh export --verbose
```

Expected: All records exported to `export/*.csv`

### Scenario 2: Incremental Sync (New Records Only)

1. Run initial export (above)
2. Add new data:

```bash
./scripts/add-new-data.sh
```

3. Run export again:

```bash
./run-export.sh export --verbose
```

Expected: Only new records in CSVs, `state.json` updated

### Scenario 3: Incremental Sync (Modified Records)

1. Modify existing records:

```bash
./scripts/modify-data.sh
```

2. Run export:

```bash
./run-export.sh export --verbose
```

Expected: Modified records in CSVs

### Scenario 4: Inactive Entities

The `archive.entity` entity is set to `active: false` in `state.json`:

```bash
./run-export.sh export --verbose
```

Expected: `archive.entity` is skipped

### Scenario 5: Dry Run Validation

Validate without exporting:

```bash
./run-export.sh export --dry-run
```

### Scenario 6: Connection Test

Test database connection only:

```bash
./run-export.sh validate --test-connection
```

## Manual SQL Testing

Connect directly to test queries:

```bash
# Using Docker (recommended)
docker exec -it ora2csv-oracle sqlplus -s ora2csv/ora2csv_pass

# Or connect from another container
docker run -it --rm --network ora2csv_e2e gvenzl/oracle-xe:21-slim-faststart sqlplus ora2csv/ora2csv_pass@//ora2csv-oracle:1521/ORCL
```

Sample query with bind variables:

```sql
-- Test the products export query
VAR startDate VARCHAR2(50) := '2025-01-01T00:00:00';
VAR tillDate VARCHAR2(50) := '2025-12-31T23:59:59';

SELECT
    id, name, sku, category, price, quantity,
    TO_CHAR(created, 'YYYY-MM-DD"T"HH24:MI:SS') as created,
    TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM crm.products
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY updated ASC;
```

## Test Data Schema

### crm.products
- `id` - Primary key
- `name`, `sku`, `category` - Product info
- `price`, `quantity` - Numeric fields
- `created`, `updated` - Timestamps for incremental sync

### crm.orders
- `id` - Primary key
- `product_id` - Foreign key to products
- `customer_name`, `email` - Customer info
- `status` - Order status (PENDING, PROCESSING, SHIPPED, DELIVERED, CANCELLED)
- `total_amount` - Order total
- `created`, `updated` - Timestamps

### crm.customers
- `id` - Primary key
- `first_name`, `last_name`, `email` - Customer info
- `phone`, `city`, `country` - Contact info
- `created`, `updated` - Timestamps

### archive.entity
- `id` - Primary key
- `entity_type`, `entity_name` - Entity info
- `status` - Entity status
- `created`, `updated` - Timestamps

## Cleanup

Stop and remove containers:

```bash
docker compose down
```

Remove volumes (deletes all data):

```bash
docker compose down -v
```

Clean export files:

```bash
rm -rf export/*
```

## Troubleshooting

### Connection Refused

Oracle takes time to start. Check logs:

```bash
docker compose logs oracle
```

Wait for `DATABASE IS READY TO USE!` message.

### ORA-12154: TNS:could not resolve the connect identifier

Verify service name:

```bash
docker exec ora2csv-oracle printenv ORACLE_DATABASE
```

### Invalid Username/Password

Reset credentials in `docker-compose.yml` and restart:

```bash
docker compose down
docker compose up -d
```

### Permission Denied on Tables

Grant permissions:

```sql
GRANT SELECT ON crm.products TO ora2csv;
GRANT SELECT ON crm.orders TO ora2csv;
GRANT SELECT ON crm.customers TO ora2csv;
GRANT SELECT ON archive.entity TO ora2csv;
```

## Environment Variables

The `run-export.sh` script sets these automatically:

| Variable               | Value          |
| ---------------------- | -------------- |
| `ORA2CSV_DB_PASSWORD`  | `ora2csv_pass` |
| `ORA2CSV_DB_HOST`      | `localhost`    |
| `ORA2CSV_DB_PORT`      | `1521`         |
| `ORA2CSV_DB_SERVICE`   | `ORCL`         |
| `ORA2CSV_DB_USER`      | `ora2csv`      |

Override as needed:

```bash
export ORA2CSV_DB_HOST=127.0.0.1
./run-export.sh export
```
