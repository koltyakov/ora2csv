#!/bin/bash
# Helper script to run ora2csv with E2E test database
# Usage: ./run-export.sh [ora2csv args...]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# E2E directory
cd "$SCRIPT_DIR"

# Check if ora2csv binary exists
BINARY="$PROJECT_ROOT/bin/ora2csv"
if [[ ! -f "$BINARY" ]]; then
    echo "ora2csv binary not found at $BINARY"
    echo "Run 'make build' from project root first."
    exit 1
fi

# Set environment variables for E2E database
export ORASYSTEMPASS="ora2csv_pass"
export ORA2CSV_DB_HOST="localhost"
export ORA2CSV_DB_PORT="1521"
export ORA2CSV_DB_SERVICE="ORCL"
export ORA2CSV_DB_USER="ora2csv"
export ORA2CSV_STATE_FILE="$SCRIPT_DIR/state.json"
export ORA2CSV_SQL_DIR="$SCRIPT_DIR/sql"
export ORA2CSV_EXPORT_DIR="$SCRIPT_DIR/export"

echo "================================================"
echo "ora2csv E2E Test Run"
echo "================================================"
echo "Binary: $BINARY"
echo "Host: $ORA2CSV_DB_HOST:$ORA2CSV_DB_PORT/$ORA2CSV_DB_SERVICE"
echo "User: $ORA2CSV_DB_USER"
echo "State: $ORA2CSV_STATE_FILE"
echo "SQL: $ORA2CSV_SQL_DIR"
echo "Export: $ORA2CSV_EXPORT_DIR"
echo "================================================"
echo ""

# Run ora2csv with provided arguments
"$BINARY" "$@"

echo ""
echo "================================================"
echo "Export complete! Check $ORA2CSV_EXPORT_DIR"
echo "================================================"
