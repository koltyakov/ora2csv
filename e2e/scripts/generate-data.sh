#!/bin/bash
# Generate random test data for ora2csv E2E testing
# Usage: ./scripts/generate-data.sh [count]
#   count: number of products to generate (default: 100)
#
# This script uses Docker exec to run sqlplus inside the Oracle container
# No Oracle Instant Client installation required on the host

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Default values
PRODUCT_COUNT=${1:-1000}
ORDERS_PER_PRODUCT=5
CUSTOMER_COUNT=$((PRODUCT_COUNT / 2))

# Container name
CONTAINER_NAME=${CONTAINER_NAME:-ora2csv-oracle}
ORACLE_USER=${ORACLE_USER:-ora2csv}

echo "================================================"
echo "ora2csv E2E Test Data Generator"
echo "================================================"
echo "Products: $PRODUCT_COUNT"
echo "Customers: $CUSTOMER_COUNT"
echo "Orders per product: $ORDERS_PER_PRODUCT"
echo "Total orders: $((PRODUCT_COUNT * ORDERS_PER_PRODUCT))"
echo "Archive entities: $((PRODUCT_COUNT / 5))"
echo "Container: $CONTAINER_NAME"
echo "================================================"

# Check if Docker container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Oracle container '$CONTAINER_NAME' is not running."
    echo "Start it with: cd $E2E_DIR && docker compose up -d"
    exit 1
fi

# Generate data using SQL via Docker exec
echo ""
echo "Generating test data..."

docker exec -i "$CONTAINER_NAME" sqlplus -s "$ORACLE_USER/ora2csv_pass@//localhost:1521/ORCL" << EOF
SET DEFINE OFF
SET SERVEROUTPUT ON
SET FEEDBACK OFF
SET PAGESIZE 0
SET HEADING OFF
DECLARE
    v_categories SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'Electronics', 'Clothing', 'Books', 'Home & Garden', 'Sports',
        'Toys', 'Food', 'Automotive', 'Health', 'Office'
    );
    v_cities SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'New York', 'Los Angeles', 'Chicago', 'Houston', 'Phoenix',
        'Philadelphia', 'San Antonio', 'San Diego', 'Dallas', 'San Jose'
    );
    v_countries SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'USA', 'Canada', 'UK', 'Germany', 'France', 'Japan', 'Australia'
    );
    v_statuses SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'PENDING', 'PROCESSING', 'SHIPPED', 'DELIVERED', 'CANCELLED'
    );
    v_first_names SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'John', 'Jane', 'Bob', 'Alice', 'Charlie', 'Diana', 'Edward', 'Fiona'
    );
    v_last_names SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'Smith', 'Johnson', 'Williams', 'Brown', 'Jones', 'Garcia', 'Miller'
    );
    v_inserted PLS_INTEGER := 0;
    v_product_count CONSTANT PLS_INTEGER := ${PRODUCT_COUNT};
    v_customer_count CONSTANT PLS_INTEGER := ${CUSTOMER_COUNT};
    v_orders_count PLS_INTEGER;
BEGIN
    -- Calculate orders count
    v_orders_count := v_product_count * ${ORDERS_PER_PRODUCT};

    -- Switch to crm
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';

    -- Generate Products
    FOR i IN 1..v_product_count LOOP
        INSERT INTO crm.products (
            id, name, sku, category, price, quantity, created, updated
        ) VALUES (
            crm.seq_products.NEXTVAL,
            'Product ' || i,
            'SKU-' || LPAD(i, 5, '0'),
            v_categories(FLOOR(DBMS_RANDOM.VALUE(1, v_categories.COUNT + 1))),
            ROUND(DBMS_RANDOM.VALUE(10, 500), 2),
            FLOOR(DBMS_RANDOM.VALUE(0, 1000)),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30)
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;
    DBMS_OUTPUT.PUT_LINE('Generated ' || v_inserted || ' products');
    v_inserted := 0;

    -- Generate Customers
    FOR i IN 1..v_customer_count LOOP
        INSERT INTO crm.customers (
            id, first_name, last_name, email, phone, city, country, created, updated
        ) VALUES (
            crm.seq_customers.NEXTVAL,
            v_first_names(FLOOR(DBMS_RANDOM.VALUE(1, v_first_names.COUNT + 1))),
            v_last_names(FLOOR(DBMS_RANDOM.VALUE(1, v_last_names.COUNT + 1))),
            'customer' || i || '@example.com',
            '+1-' || FLOOR(DBMS_RANDOM.VALUE(100, 999)) || '-' ||
                FLOOR(DBMS_RANDOM.VALUE(100, 999)) || '-' ||
                FLOOR(DBMS_RANDOM.VALUE(1000, 9999)),
            v_cities(FLOOR(DBMS_RANDOM.VALUE(1, v_cities.COUNT + 1))),
            v_countries(FLOOR(DBMS_RANDOM.VALUE(1, v_countries.COUNT + 1))),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30)
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;
    DBMS_OUTPUT.PUT_LINE('Generated ' || v_inserted || ' customers');
    v_inserted := 0;

    -- Generate Orders
    FOR i IN 1..v_orders_count LOOP
        INSERT INTO crm.orders (
            id, product_id, customer_name, email, status, total_amount, created, updated
        ) VALUES (
            crm.seq_orders.NEXTVAL,
            FLOOR(DBMS_RANDOM.VALUE(1, v_product_count)),
            'Customer ' || FLOOR(DBMS_RANDOM.VALUE(1, v_customer_count)),
            'customer' || FLOOR(DBMS_RANDOM.VALUE(1, v_customer_count)) || '@example.com',
            v_statuses(FLOOR(DBMS_RANDOM.VALUE(1, v_statuses.COUNT + 1))),
            ROUND(DBMS_RANDOM.VALUE(20, 1000), 2),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30)
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;
    DBMS_OUTPUT.PUT_LINE('Generated ' || v_inserted || ' orders');
    v_inserted := 0;

    -- Generate Archive data
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = archive';
    FOR i IN 1..FLOOR(v_product_count / 5) LOOP
        INSERT INTO archive.entity (
            id, entity_type, entity_name, status, created, updated
        ) VALUES (
            archive.seq_entity.NEXTVAL,
            CASE MOD(i, 3)
                WHEN 0 THEN 'PRODUCT'
                WHEN 1 THEN 'ORDER'
                ELSE 'CUSTOMER'
            END,
            'Archived Entity ' || i,
            CASE MOD(i, 2)
                WHEN 0 THEN 'ACTIVE'
                ELSE 'ARCHIVED'
            END,
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30),
            SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30)
        );
        v_inserted := v_inserted + SQL%ROWCOUNT;
    END LOOP;
    DBMS_OUTPUT.PUT_LINE('Generated ' || v_inserted || ' archive entities');

    COMMIT;
END;
/

EXIT
EOF

echo ""
echo "================================================"
echo "Data generation complete!"
echo "================================================"
