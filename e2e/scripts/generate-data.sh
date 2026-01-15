#!/bin/bash
# Generate random test data for ora2csv E2E testing
# Usage: ./scripts/generate-data.sh [product_count] [orders_per_product] [customer_count]
#   product_count: number of products to generate (default: 1000)
#   orders_per_product: orders per product (default: 5)
#   customer_count: number of customers (default: product_count/2)
#
# Optimized with FORALL bulk binding for high-performance inserts
# Can handle 1M+ rows efficiently
#
# This script uses Docker exec to run sqlplus inside the Oracle container
# No Oracle Instant Client installation required on the host

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(dirname "$SCRIPT_DIR")"

# Default values - support millions of records
PRODUCT_COUNT=${1:-1000}
ORDERS_PER_PRODUCT=${2:-5}
CUSTOMER_COUNT=${3:-$((${PRODUCT_COUNT:-1000} / 2))}

# Calculate totals
TOTAL_ORDERS=$((PRODUCT_COUNT * ORDERS_PER_PRODUCT))
ARCHIVE_COUNT=$((PRODUCT_COUNT / 5))
TOTAL_RECORDS=$((PRODUCT_COUNT + CUSTOMER_COUNT + TOTAL_ORDERS + ARCHIVE_COUNT))

# Container name
CONTAINER_NAME=${CONTAINER_NAME:-ora2csv-oracle}
ORACLE_USER=${ORACLE_USER:-ora2csv}

echo "================================================"
echo "ora2csv E2E Test Data Generator (Bulk Optimized)"
echo "================================================"
echo "Products: $PRODUCT_COUNT"
echo "Customers: $CUSTOMER_COUNT"
echo "Orders per product: $ORDERS_PER_PRODUCT"
echo "Total orders: $TOTAL_ORDERS"
echo "Archive entities: $ARCHIVE_COUNT"
echo "Total records: $TOTAL_RECORDS"
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
echo "Generating test data with bulk inserts..."

# Calculate elapsed time
START_TIME=$(date +%s)

docker exec -i "$CONTAINER_NAME" sqlplus -s "$ORACLE_USER/ora2csv_pass@//localhost:1521/ORCL" << EOF
SET DEFINE OFF
SET SERVEROUTPUT ON
SET FEEDBACK OFF
SET PAGESIZE 0
SET HEADING OFF
DECLARE
    -- Lookup collections
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

    -- Record type for products
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
    TYPE t_product_tab IS TABLE OF t_product_rec;

    -- Record type for customers
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
    TYPE t_customer_tab IS TABLE OF t_customer_rec;

    -- Record type for orders
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
    TYPE t_order_tab IS TABLE OF t_order_rec;

    -- Record type for archive entities
    TYPE t_archive_rec IS RECORD (
        id         PLS_INTEGER,
        entity_type VARCHAR2(20),
        entity_name VARCHAR2(100),
        status      VARCHAR2(20),
        created     TIMESTAMP,
        updated     TIMESTAMP
    );
    TYPE t_archive_tab IS TABLE OF t_archive_rec;

    -- Collection variables
    v_products  t_product_tab := t_product_tab();
    v_customers t_customer_tab := t_customer_tab();
    v_orders    t_order_tab := t_order_tab();
    v_archives  t_archive_tab := t_archive_tab();

    -- Constants and variables
    c_batch_size CONSTANT PLS_INTEGER := 25000;  -- Optimal batch size
    v_product_count CONSTANT PLS_INTEGER := ${PRODUCT_COUNT};
    v_customer_count CONSTANT PLS_INTEGER := ${CUSTOMER_COUNT};
    v_orders_count CONSTANT PLS_INTEGER := ${TOTAL_ORDERS};
    v_archive_count CONSTANT PLS_INTEGER := ${ARCHIVE_COUNT};

    v_start_time NUMBER;
    v_elapsed   NUMBER;

    -- Helper function to get random item from collection
    FUNCTION random_category RETURN VARCHAR2 IS
    BEGIN
        RETURN v_categories(FLOOR(DBMS_RANDOM.VALUE(1, v_categories.COUNT + 1)));
    END;

    FUNCTION random_city RETURN VARCHAR2 IS
    BEGIN
        RETURN v_cities(FLOOR(DBMS_RANDOM.VALUE(1, v_cities.COUNT + 1)));
    END;

    FUNCTION random_country RETURN VARCHAR2 IS
    BEGIN
        RETURN v_countries(FLOOR(DBMS_RANDOM.VALUE(1, v_countries.COUNT + 1)));
    END;

    FUNCTION random_status RETURN VARCHAR2 IS
    BEGIN
        RETURN v_statuses(FLOOR(DBMS_RANDOM.VALUE(1, v_statuses.COUNT + 1)));
    END;

    FUNCTION random_first_name RETURN VARCHAR2 IS
    BEGIN
        RETURN v_first_names(FLOOR(DBMS_RANDOM.VALUE(1, v_first_names.COUNT + 1)));
    END;

    FUNCTION random_last_name RETURN VARCHAR2 IS
    BEGIN
        RETURN v_last_names(FLOOR(DBMS_RANDOM.VALUE(1, v_last_names.COUNT + 1)));
    END;

    -- Bulk insert procedures (no SAVE EXCEPTIONS - fail fast on error)
    PROCEDURE flush_products IS
    BEGIN
        IF v_products.COUNT > 0 THEN
            FORALL i IN v_products.FIRST..v_products.LAST
                INSERT INTO crm.products (id, name, sku, category, price, quantity, created, updated)
                VALUES (
                    v_products(i).id,
                    v_products(i).name,
                    v_products(i).sku,
                    v_products(i).category,
                    v_products(i).price,
                    v_products(i).quantity,
                    v_products(i).created,
                    v_products(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('  Inserted ' || v_products.COUNT || ' products');
            v_products.DELETE;
        END IF;
    END;

    PROCEDURE flush_customers IS
    BEGIN
        IF v_customers.COUNT > 0 THEN
            FORALL i IN v_customers.FIRST..v_customers.LAST
                INSERT INTO crm.customers (id, first_name, last_name, email, phone, city, country, created, updated)
                VALUES (
                    v_customers(i).id,
                    v_customers(i).first_name,
                    v_customers(i).last_name,
                    v_customers(i).email,
                    v_customers(i).phone,
                    v_customers(i).city,
                    v_customers(i).country,
                    v_customers(i).created,
                    v_customers(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('  Inserted ' || v_customers.COUNT || ' customers');
            v_customers.DELETE;
        END IF;
    END;

    PROCEDURE flush_orders IS
    BEGIN
        IF v_orders.COUNT > 0 THEN
            FORALL i IN v_orders.FIRST..v_orders.LAST
                INSERT INTO crm.orders (id, product_id, customer_name, email, status, total_amount, created, updated)
                VALUES (
                    v_orders(i).id,
                    v_orders(i).product_id,
                    v_orders(i).customer_name,
                    v_orders(i).email,
                    v_orders(i).status,
                    v_orders(i).total_amount,
                    v_orders(i).created,
                    v_orders(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('  Inserted ' || v_orders.COUNT || ' orders');
            v_orders.DELETE;
        END IF;
    END;

    PROCEDURE flush_archives IS
    BEGIN
        IF v_archives.COUNT > 0 THEN
            FORALL i IN v_archives.FIRST..v_archives.LAST
                INSERT INTO archive.entity (id, entity_type, entity_name, status, created, updated)
                VALUES (
                    v_archives(i).id,
                    v_archives(i).entity_type,
                    v_archives(i).entity_name,
                    v_archives(i).status,
                    v_archives(i).created,
                    v_archives(i).updated
                );
            DBMS_OUTPUT.PUT_LINE('  Inserted ' || v_archives.COUNT || ' archive entities');
            v_archives.DELETE;
        END IF;
    END;

BEGIN
    v_start_time := DBMS_UTILITY.GET_TIME;

    -- Switch to crm schema
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = crm';

    DBMS_OUTPUT.PUT_LINE('Generating products...');
    FOR i IN 1..v_product_count LOOP
        v_products.EXTEND;
        v_products(v_products.LAST).id := crm.seq_products.NEXTVAL;
        v_products(v_products.LAST).name := 'Product ' || i;
        v_products(v_products.LAST).sku := 'SKU-' || LPAD(crm.seq_products.CURRVAL, 7, '0');
        v_products(v_products.LAST).category := random_category;
        v_products(v_products.LAST).price := ROUND(DBMS_RANDOM.VALUE(10, 500), 2);
        v_products(v_products.LAST).quantity := FLOOR(DBMS_RANDOM.VALUE(0, 1000));
        v_products(v_products.LAST).created := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);
        v_products(v_products.LAST).updated := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);

        IF MOD(i, c_batch_size) = 0 THEN
            flush_products;
        END IF;
    END LOOP;
    flush_products;

    DBMS_OUTPUT.PUT_LINE('Generating customers...');
    FOR i IN 1..v_customer_count LOOP
        v_customers.EXTEND;
        v_customers(v_customers.LAST).id := crm.seq_customers.NEXTVAL;
        v_customers(v_customers.LAST).first_name := random_first_name;
        v_customers(v_customers.LAST).last_name := random_last_name;
        v_customers(v_customers.LAST).email := 'customer' || crm.seq_customers.CURRVAL || '@example.com';
        v_customers(v_customers.LAST).phone := '+1-' ||
            FLOOR(DBMS_RANDOM.VALUE(100, 999)) || '-' ||
            LPAD(FLOOR(DBMS_RANDOM.VALUE(100, 999)), 3, '0') || '-' ||
            LPAD(FLOOR(DBMS_RANDOM.VALUE(1000, 9999)), 4, '0');
        v_customers(v_customers.LAST).city := random_city;
        v_customers(v_customers.LAST).country := random_country;
        v_customers(v_customers.LAST).created := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);
        v_customers(v_customers.LAST).updated := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);

        IF MOD(i, c_batch_size) = 0 THEN
            flush_customers;
        END IF;
    END LOOP;
    flush_customers;

    DBMS_OUTPUT.PUT_LINE('Generating orders...');
    FOR i IN 1..v_orders_count LOOP
        v_orders.EXTEND;
        v_orders(v_orders.LAST).id := crm.seq_orders.NEXTVAL;
        v_orders(v_orders.LAST).product_id := FLOOR(DBMS_RANDOM.VALUE(1, v_product_count + 1));
        v_orders(v_orders.LAST).customer_name := 'Customer ' || FLOOR(DBMS_RANDOM.VALUE(1, v_customer_count + 1));
        v_orders(v_orders.LAST).email := 'customer' || FLOOR(DBMS_RANDOM.VALUE(1, v_customer_count + 1)) || '@example.com';
        v_orders(v_orders.LAST).status := random_status;
        v_orders(v_orders.LAST).total_amount := ROUND(DBMS_RANDOM.VALUE(20, 1000), 2);
        v_orders(v_orders.LAST).created := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);
        v_orders(v_orders.LAST).updated := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);

        IF MOD(i, c_batch_size) = 0 THEN
            flush_orders;
        END IF;
    END LOOP;
    flush_orders;

    -- Generate Archive data
    EXECUTE IMMEDIATE 'ALTER SESSION SET CURRENT_SCHEMA = archive';
    DBMS_OUTPUT.PUT_LINE('Generating archive entities...');
    FOR i IN 1..v_archive_count LOOP
        v_archives.EXTEND;
        v_archives(v_archives.LAST).id := archive.seq_entity.NEXTVAL;
        v_archives(v_archives.LAST).entity_type := CASE MOD(i, 3)
            WHEN 0 THEN 'PRODUCT'
            WHEN 1 THEN 'ORDER'
            ELSE 'CUSTOMER'
        END;
        v_archives(v_archives.LAST).entity_name := 'Archived Entity ' || i;
        v_archives(v_archives.LAST).status := CASE MOD(i, 2)
            WHEN 0 THEN 'ACTIVE'
            ELSE 'ARCHIVED'
        END;
        v_archives(v_archives.LAST).created := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);
        v_archives(v_archives.LAST).updated := SYSTIMESTAMP - DBMS_RANDOM.VALUE(0, 30);

        IF MOD(i, c_batch_size) = 0 THEN
            flush_archives;
        END IF;
    END LOOP;
    flush_archives;

    COMMIT;

    v_elapsed := (DBMS_UTILITY.GET_TIME - v_start_time) / 100;
    DBMS_OUTPUT.PUT_LINE('================================================');
    DBMS_OUTPUT.PUT_LINE('Data generation complete!');
    DBMS_OUTPUT.PUT_LINE('Total records: ' || (v_product_count + v_customer_count + v_orders_count + v_archive_count));
    DBMS_OUTPUT.PUT_LINE('Elapsed time: ' || v_elapsed || ' seconds');
    DBMS_OUTPUT.PUT_LINE('Throughput: ' || ROUND((v_product_count + v_customer_count + v_orders_count + v_archive_count) / GREATEST(v_elapsed, 0.01)) || ' records/sec');
    DBMS_OUTPUT.PUT_LINE('================================================');
END;
/

EXIT
EOF

END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "================================================"
echo "Data generation complete!"
echo "Wall clock time: ${ELAPSED}s"
echo "================================================"
