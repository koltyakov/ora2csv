#!/bin/bash
# Modify existing data to test incremental sync
# This script updates random records with new timestamps
#
# This script uses Docker exec to run sqlplus inside the Oracle container
# No Oracle Instant Client installation required on the host

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Container name
CONTAINER_NAME=${CONTAINER_NAME:-ora2csv-oracle}
ORACLE_USER=${ORACLE_USER:-ora2csv}

echo "================================================"
echo "ora2csv E2E - Modify Data for Incremental Sync"
echo "================================================"
echo ""

# Check if Docker container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Oracle container '$CONTAINER_NAME' is not running."
    echo "Start it with: cd $E2E_DIR && docker compose up -d"
    exit 1
fi

docker exec -i "$CONTAINER_NAME" sqlplus -s "$ORACLE_USER/ora2csv_pass@//localhost:1521/ORCL" << 'EOF'
SET SERVEROUTPUT ON
SET FEEDBACK OFF
SET PAGESIZE 0
SET HEADING OFF
DECLARE
    v_updated PLS_INTEGER := 0;
BEGIN
    -- Update some products
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';

    FOR i IN 1..20 LOOP
        UPDATE crm.products
        SET quantity = FLOOR(DBMS_RANDOM.VALUE(0, 1000)),
            price = ROUND(DBMS_RANDOM.VALUE(10, 500), 2),
            updated = SYSTIMESTAMP
        WHERE id = FLOOR(DBMS_RANDOM.VALUE(1, 1000))
        AND ROWNUM = 1;
        v_updated := v_updated + SQL%ROWCOUNT;
    END LOOP;

    -- Update some orders (change status)
    FOR i IN 1..20 LOOP
        UPDATE crm.orders
        SET status = CASE MOD(i, 5)
            WHEN 0 THEN 'PENDING'
            WHEN 1 THEN 'PROCESSING'
            WHEN 2 THEN 'SHIPPED'
            WHEN 3 THEN 'DELIVERED'
            ELSE 'CANCELLED'
        END,
        updated = SYSTIMESTAMP
        WHERE id = FLOOR(DBMS_RANDOM.VALUE(1, 5000))
        AND ROWNUM = 1;
        v_updated := v_updated + SQL%ROWCOUNT;
    END LOOP;

    -- Update some customers
    FOR i IN 1..20 LOOP
        UPDATE crm.customers
        SET city = CASE MOD(i, 10)
            WHEN 0 THEN 'New York'
            WHEN 1 THEN 'Los Angeles'
            WHEN 2 THEN 'Chicago'
            WHEN 3 THEN 'Houston'
            WHEN 4 THEN 'Phoenix'
            WHEN 5 THEN 'Philadelphia'
            WHEN 6 THEN 'San Antonio'
            WHEN 7 THEN 'San Diego'
            WHEN 8 THEN 'Dallas'
            ELSE 'San Jose'
        END,
        updated = SYSTIMESTAMP
        WHERE id = FLOOR(DBMS_RANDOM.VALUE(1, 500))
        AND ROWNUM = 1;
        v_updated := v_updated + SQL%ROWCOUNT;
    END LOOP;

    COMMIT;
    DBMS_OUTPUT.PUT_LINE('Updated ' || v_updated || ' records');
END;
/

EXIT
EOF
