#!/bin/bash
# Modify existing data to test incremental sync
# This script updates random records with new timestamps
# Optimized with bulk UPDATE operations
#
# This script uses Docker exec to run sqlplus inside the Oracle container
# No Oracle Instant Client installation required on the host

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Container name
CONTAINER_NAME=${CONTAINER_NAME:-ora2csv-oracle}
ORACLE_USER=${ORACLE_USER:-ora2csv}

# Configurable update counts
UPDATE_PRODUCTS=${UPDATE_PRODUCTS:-20}
UPDATE_ORDERS=${UPDATE_ORDERS:-20}
UPDATE_CUSTOMERS=${UPDATE_CUSTOMERS:-20}

echo "================================================"
echo "ora2csv E2E - Modify Data for Incremental Sync"
echo "================================================"
echo "Updates per entity: products=$UPDATE_PRODUCTS, orders=$UPDATE_ORDERS, customers=$UPDATE_CUSTOMERS"
echo "================================================"
echo ""

# Check if Docker container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Oracle container '$CONTAINER_NAME' is not running."
    echo "Start it with: cd $E2E_DIR && docker compose up -d"
    exit 1
fi

docker exec -i "$CONTAINER_NAME" sqlplus -s "$ORACLE_USER/ora2csv_pass@//localhost:1521/ORCL" << EOF
SET SERVEROUTPUT ON
SET FEEDBACK OFF
SET PAGESIZE 0
SET HEADING OFF
DECLARE
    -- Arrays to hold IDs for bulk update
    TYPE t_id_tab IS TABLE OF PLS_INTEGER;
    v_product_ids t_id_tab := t_id_tab();
    v_order_ids    t_id_tab := t_id_tab();
    v_customer_ids t_id_tab := t_id_tab();

    -- Arrays for update values (parallel with IDs)
    TYPE t_number_tab IS TABLE OF NUMBER;
    TYPE t_varchar2_tab IS TABLE OF VARCHAR2(100);
    TYPE t_timestamp_tab IS TABLE OF TIMESTAMP;

    v_quantities   t_number_tab := t_number_tab();
    v_prices       t_number_tab := t_number_tab();
    v_order_stats  t_varchar2_tab := t_varchar2_tab();
    v_cities       t_varchar2_tab := t_varchar2_tab();

    -- Constants
    v_product_updates CONSTANT PLS_INTEGER := ${UPDATE_PRODUCTS};
    v_order_updates CONSTANT PLS_INTEGER := ${UPDATE_ORDERS};
    v_customer_updates CONSTANT PLS_INTEGER := ${UPDATE_CUSTOMERS};

    v_updated PLS_INTEGER := 0;

    -- Status options
    v_statuses SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'PENDING', 'PROCESSING', 'SHIPPED', 'DELIVERED', 'CANCELLED'
    );
    v_cities_list SYS.ODCIVARCHAR2LIST := SYS.ODCIVARCHAR2LIST(
        'New York', 'Los Angeles', 'Chicago', 'Houston', 'Phoenix',
        'Philadelphia', 'San Antonio', 'San Diego', 'Dallas', 'San Jose'
    );

    -- Get max IDs from tables
    v_max_product_id PLS_INTEGER;
    v_max_order_id PLS_INTEGER;
    v_max_customer_id PLS_INTEGER;

BEGIN
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';

    -- Get current max IDs
    SELECT NVL(MAX(id), 0) INTO v_max_product_id FROM crm.products;
    SELECT NVL(MAX(id), 0) INTO v_max_order_id FROM crm.orders;
    SELECT NVL(MAX(id), 0) INTO v_max_customer_id FROM crm.customers;

    DBMS_OUTPUT.PUT_LINE('Max IDs - Products: ' || v_max_product_id ||
                        ', Orders: ' || v_max_order_id ||
                        ', Customers: ' || v_max_customer_id);

    -- Prepare product updates (select random IDs)
    FOR i IN 1..LEAST(v_product_updates, v_max_product_id) LOOP
        v_product_ids.EXTEND;
        v_product_ids(v_product_ids.LAST) := FLOOR(DBMS_RANDOM.VALUE(1, v_max_product_id + 1));
        v_quantities.EXTEND;
        v_quantities(v_quantities.LAST) := FLOOR(DBMS_RANDOM.VALUE(0, 1000));
        v_prices.EXTEND;
        v_prices(v_prices.LAST) := ROUND(DBMS_RANDOM.VALUE(10, 500), 2);
    END LOOP;

    -- Bulk update products
    IF v_product_ids.COUNT > 0 THEN
        FORALL i IN v_product_ids.FIRST..v_product_ids.LAST SAVE EXCEPTIONS
            UPDATE crm.products
            SET quantity = v_quantities(i),
                price = v_prices(i),
                updated = SYSTIMESTAMP
            WHERE id = v_product_ids(i);
        v_updated := v_updated + SQL%ROWCOUNT;
    END IF;

    -- Prepare order updates
    FOR i IN 1..LEAST(v_order_updates, v_max_order_id) LOOP
        v_order_ids.EXTEND;
        v_order_ids(v_order_ids.LAST) := FLOOR(DBMS_RANDOM.VALUE(1, v_max_order_id + 1));
        v_order_stats.EXTEND;
        v_order_stats(v_order_stats.LAST) := v_statuses(
            FLOOR(DBMS_RANDOM.VALUE(1, v_statuses.COUNT + 1))
        );
    END LOOP;

    -- Bulk update orders
    IF v_order_ids.COUNT > 0 THEN
        FORALL i IN v_order_ids.FIRST..v_order_ids.LAST SAVE EXCEPTIONS
            UPDATE crm.orders
            SET status = v_order_stats(i),
                updated = SYSTIMESTAMP
            WHERE id = v_order_ids(i);
        v_updated := v_updated + SQL%ROWCOUNT;
    END IF;

    -- Prepare customer updates
    FOR i IN 1..LEAST(v_customer_updates, v_max_customer_id) LOOP
        v_customer_ids.EXTEND;
        v_customer_ids(v_customer_ids.LAST) := FLOOR(DBMS_RANDOM.VALUE(1, v_max_customer_id + 1));
        v_cities.EXTEND;
        v_cities(v_cities.LAST) := v_cities_list(
            FLOOR(DBMS_RANDOM.VALUE(1, v_cities_list.COUNT + 1))
        );
    END LOOP;

    -- Bulk update customers
    IF v_customer_ids.COUNT > 0 THEN
        FORALL i IN v_customer_ids.FIRST..v_customer_ids.LAST SAVE EXCEPTIONS
            UPDATE crm.customers
            SET city = v_cities(i),
                updated = SYSTIMESTAMP
            WHERE id = v_customer_ids(i);
        v_updated := v_updated + SQL%ROWCOUNT;
    END IF;

    COMMIT;
    DBMS_OUTPUT.PUT_LINE('Updated ' || v_updated || ' records');
END;
/

EXIT
EOF

