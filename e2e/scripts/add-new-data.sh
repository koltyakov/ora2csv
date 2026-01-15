#!/bin/bash
# Add new records to test incremental sync (new data only)
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
echo "ora2csv E2E - Add New Data"
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
    v_inserted PLS_INTEGER := 0;
BEGIN
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';

    -- Add new products
    FOR i IN 1..100 LOOP
        INSERT INTO crm.products (
            id, name, sku, category, price, quantity, created, updated
        ) VALUES (
            crm.seq_products.NEXTVAL,
            'New Product ' || i,
            'SKU-NEW-' || LPAD(crm.seq_products.CURRVAL, 5, '0'),
            'Electronics',
            ROUND(DBMS_RANDOM.VALUE(10, 500), 2),
            FLOOR(DBMS_RANDOM.VALUE(0, 100)),
            SYSTIMESTAMP,
            SYSTIMESTAMP
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;

    -- Add new customers
    FOR i IN 1..50 LOOP
        INSERT INTO crm.customers (
            id, first_name, last_name, email, phone, city, country, created, updated
        ) VALUES (
            crm.seq_customers.NEXTVAL,
            'New',
            'Customer ' || i,
            'new.customer' || i || '@example.com',
            '+1-555-' || FLOOR(DBMS_RANDOM.VALUE(100, 999)) || '-' ||
                FLOOR(DBMS_RANDOM.VALUE(1000, 9999)),
            'Seattle',
            'USA',
            SYSTIMESTAMP,
            SYSTIMESTAMP
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;

    -- Add new orders
    FOR i IN 1..150 LOOP
        INSERT INTO crm.orders (
            id, product_id, customer_name, email, status, total_amount, created, updated
        ) VALUES (
            crm.seq_orders.NEXTVAL,
            FLOOR(DBMS_RANDOM.VALUE(1, 100)),
            'New Customer ' || FLOOR(DBMS_RANDOM.VALUE(1, 5)),
            'new.customer' || FLOOR(DBMS_RANDOM.VALUE(1, 5)) || '@example.com',
            'PENDING',
            ROUND(DBMS_RANDOM.VALUE(20, 500), 2),
            SYSTIMESTAMP,
            SYSTIMESTAMP
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;

    COMMIT;
    DBMS_OUTPUT.PUT_LINE('Added ' || v_inserted || ' new rows');
END;
/

EXIT
EOF
