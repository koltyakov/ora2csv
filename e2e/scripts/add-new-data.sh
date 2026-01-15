#!/bin/bash
# Add new records to test incremental sync (new data only)
# Optimized with FORALL bulk binding for high-performance inserts
#
# This script uses Docker exec to run sqlplus inside the Oracle container
# No Oracle Instant Client installation required on the host

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Container name
CONTAINER_NAME=${CONTAINER_NAME:-ora2csv-oracle}
ORACLE_USER=${ORACLE_USER:-ora2csv}

# Configurable batch sizes
NEW_PRODUCTS=${NEW_PRODUCTS:-100}
NEW_CUSTOMERS=${NEW_CUSTOMERS:-50}
NEW_ORDERS=${NEW_ORDERS:-150}

echo "================================================"
echo "ora2csv E2E - Add New Data (Bulk Optimized)"
echo "================================================"
echo "New Products: $NEW_PRODUCTS"
echo "New Customers: $NEW_CUSTOMERS"
echo "New Orders: $NEW_ORDERS"
echo "Container: $CONTAINER_NAME"
echo "================================================"
echo ""

# Check if Docker container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Oracle container '$CONTAINER_NAME' is not running."
    echo "Start it with: cd $E2E_DIR && docker compose up -d"
    exit 1
fi

echo "Adding new data using bulk inserts..."
# Small delay to ensure new timestamps are after any recent export runs
sleep 2

docker exec -i "$CONTAINER_NAME" sqlplus -s "$ORACLE_USER/ora2csv_pass@//localhost:1521/ORCL" << EOF
SET SERVEROUTPUT ON
SET FEEDBACK OFF
SET PAGESIZE 0
SET HEADING OFF
DECLARE
    -- Define record types
    TYPE t_product_rec IS RECORD (
        id         PLS_INTEGER,
        name       VARCHAR2(100),
        sku        VARCHAR2(20),
        category   VARCHAR2(50),
        price      NUMBER(10,2),
        quantity   PLS_INTEGER,
        created    TIMESTAMP,
        updated    TIMESTAMP
    );
    TYPE t_customer_rec IS RECORD (
        id          PLS_INTEGER,
        first_name  VARCHAR2(50),
        last_name   VARCHAR2(50),
        email       VARCHAR2(100),
        phone       VARCHAR2(20),
        city        VARCHAR2(50),
        country     VARCHAR2(50),
        created     TIMESTAMP,
        updated     TIMESTAMP
    );
    TYPE t_order_rec IS RECORD (
        id            PLS_INTEGER,
        product_id    PLS_INTEGER,
        customer_name VARCHAR2(100),
        email         VARCHAR2(100),
        status        VARCHAR2(20),
        total_amount  NUMBER(10,2),
        created       TIMESTAMP,
        updated       TIMESTAMP
    );

    -- Define collection types
    TYPE t_product_tab IS TABLE OF t_product_rec;
    TYPE t_customer_tab IS TABLE OF t_customer_rec;
    TYPE t_order_tab IS TABLE OF t_order_rec;

    -- Collection variables
    v_products  t_product_tab := t_product_tab();
    v_customers t_customer_tab := t_customer_tab();
    v_orders    t_order_tab := t_order_tab();

    -- Constants
    c_batch_size CONSTANT PLS_INTEGER := 10000;
    v_product_count CONSTANT PLS_INTEGER := ${NEW_PRODUCTS};
    v_customer_count CONSTANT PLS_INTEGER := ${NEW_CUSTOMERS};
    v_order_count CONSTANT PLS_INTEGER := ${NEW_ORDERS};

    -- Get max IDs for offset
    v_max_product_id   PLS_INTEGER;
    v_max_customer_id  PLS_INTEGER;
    v_max_order_id     PLS_INTEGER;

    -- Procedure to bulk insert products and clear collection
    PROCEDURE flush_products IS
    BEGIN
        IF v_products.COUNT > 0 THEN
            FORALL i IN v_products.FIRST..v_products.LAST
                INSERT INTO crm.products (
                    id, name, sku, category, price, quantity, created, updated
                ) VALUES (
                    crm.seq_products.NEXTVAL,
                    v_products(i).name,
                    v_products(i).sku,
                    v_products(i).category,
                    v_products(i).price,
                    v_products(i).quantity,
                    v_products(i).created,
                    v_products(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('Inserted ' || v_products.COUNT || ' products');
            v_products.DELETE;
        END IF;
    END;

    -- Procedure to bulk insert customers and clear collection
    PROCEDURE flush_customers IS
    BEGIN
        IF v_customers.COUNT > 0 THEN
            FORALL i IN v_customers.FIRST..v_customers.LAST
                INSERT INTO crm.customers (
                    id, first_name, last_name, email, phone, city, country, created, updated
                ) VALUES (
                    crm.seq_customers.NEXTVAL,
                    v_customers(i).first_name,
                    v_customers(i).last_name,
                    v_customers(i).email,
                    v_customers(i).phone,
                    v_customers(i).city,
                    v_customers(i).country,
                    v_customers(i).created,
                    v_customers(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('Inserted ' || v_customers.COUNT || ' customers');
            v_customers.DELETE;
        END IF;
    END;

    -- Procedure to bulk insert orders and clear collection
    PROCEDURE flush_orders IS
    BEGIN
        IF v_orders.COUNT > 0 THEN
            FORALL i IN v_orders.FIRST..v_orders.LAST
                INSERT INTO crm.orders (
                    id, product_id, customer_name, email, status, total_amount, created, updated
                ) VALUES (
                    crm.seq_orders.NEXTVAL,
                    v_orders(i).product_id,
                    v_orders(i).customer_name,
                    v_orders(i).email,
                    v_orders(i).status,
                    v_orders(i).total_amount,
                    v_orders(i).created,
                    v_orders(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('Inserted ' || v_orders.COUNT || ' orders');
            v_orders.DELETE;
        END IF;
    END;

BEGIN
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';
    DBMS_OUTPUT.PUT_LINE('Starting bulk data insertion...');

    -- Get current max IDs for reference
    SELECT NVL(MAX(id), 0) INTO v_max_product_id FROM crm.products;
    SELECT NVL(MAX(id), 0) INTO v_max_customer_id FROM crm.customers;
    SELECT NVL(MAX(id), 0) INTO v_max_order_id FROM crm.orders;

    -- Build product collection
    FOR i IN 1..v_product_count LOOP
        v_products.EXTEND;
        v_products(v_products.LAST).id := i;
        v_products(v_products.LAST).name := 'New Product ' || (v_max_product_id + i);
        v_products(v_products.LAST).sku := 'SKU-NEW-' || LPAD(v_max_product_id + i, 5, '0');
        v_products(v_products.LAST).category := 'Electronics';
        v_products(v_products.LAST).price := ROUND(DBMS_RANDOM.VALUE(10, 500), 2);
        v_products(v_products.LAST).quantity := FLOOR(DBMS_RANDOM.VALUE(0, 100));
        v_products(v_products.LAST).created := SYSTIMESTAMP;
        v_products(v_products.LAST).updated := SYSTIMESTAMP;

        -- Flush when batch size reached
        IF MOD(i, c_batch_size) = 0 THEN
            flush_products;
        END IF;
    END LOOP;
    flush_products; -- Flush remaining

    -- Build customer collection
    FOR i IN 1..v_customer_count LOOP
        v_customers.EXTEND;
        v_customers(v_customers.LAST).id := i;
        v_customers(v_customers.LAST).first_name := 'New';
        v_customers(v_customers.LAST).last_name := 'Customer ' || (v_max_customer_id + i);
        v_customers(v_customers.LAST).email := 'new.customer' || (v_max_customer_id + i) || '@example.com';
        v_customers(v_customers.LAST).phone := '+1-555-' ||
            FLOOR(DBMS_RANDOM.VALUE(100, 999)) || '-' ||
            LPAD(FLOOR(DBMS_RANDOM.VALUE(1000, 9999)), 4, '0');
        v_customers(v_customers.LAST).city := 'Seattle';
        v_customers(v_customers.LAST).country := 'USA';
        v_customers(v_customers.LAST).created := SYSTIMESTAMP;
        v_customers(v_customers.LAST).updated := SYSTIMESTAMP;

        IF MOD(i, c_batch_size) = 0 THEN
            flush_customers;
        END IF;
    END LOOP;
    flush_customers;

    -- Build order collection
    FOR i IN 1..v_order_count LOOP
        v_orders.EXTEND;
        v_orders(v_orders.LAST).id := i;
        -- Use existing product IDs (random from 1 to max_product_id + new products)
        v_orders(v_orders.LAST).product_id := FLOOR(DBMS_RANDOM.VALUE(1, v_max_product_id + v_product_count + 1));
        v_orders(v_orders.LAST).customer_name := 'New Customer ' || FLOOR(DBMS_RANDOM.VALUE(1, 5));
        v_orders(v_orders.LAST).email := 'new.customer' ||
            FLOOR(DBMS_RANDOM.VALUE(1, 5)) || '@example.com';
        v_orders(v_orders.LAST).status := 'PENDING';
        v_orders(v_orders.LAST).total_amount := ROUND(DBMS_RANDOM.VALUE(20, 500), 2);
        v_orders(v_orders.LAST).created := SYSTIMESTAMP;
        v_orders(v_orders.LAST).updated := SYSTIMESTAMP;

        IF MOD(i, c_batch_size) = 0 THEN
            flush_orders;
        END IF;
    END LOOP;
    flush_orders;

    COMMIT;
    DBMS_OUTPUT.PUT_LINE('================================================');
    DBMS_OUTPUT.PUT_LINE('Total new records added: ' || (v_product_count + v_customer_count + v_order_count));
    DBMS_OUTPUT.PUT_LINE('================================================');
END;
/

EXIT
EOF
