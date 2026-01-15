-- Oracle Schema Initialization for ora2csv E2E Testing
-- This file is automatically executed when the Oracle container starts
-- for the first time (via docker-entrypoint-initdb.d)
--
-- This script must be run by SYS AS SYSDBA (default for initdb.d scripts)

SET SERVEROUTPUT ON

-- Switch to the Pluggable Database (PDB) called ORCL
-- This is where application users and tables should be created
ALTER SESSION SET CONTAINER = ORCL;

-- Create schemas
ALTER SESSION SET "_ORACLE_SCRIPT" = TRUE;

-- Create ora2csv user first (before we try to grant to it)
CREATE USER ora2csv IDENTIFIED BY ora2csv_pass;
GRANT CONNECT, RESOURCE TO ora2csv;
ALTER USER ora2csv DEFAULT TABLESPACE USERS;
ALTER USER ora2csv QUOTA UNLIMITED ON USERS;

CREATE USER crm IDENTIFIED BY crm_pass;
GRANT CONNECT, RESOURCE TO crm;
GRANT CREATE VIEW, CREATE SEQUENCE TO crm;
ALTER USER crm DEFAULT TABLESPACE USERS;
ALTER USER crm QUOTA UNLIMITED ON USERS;

CREATE USER archive IDENTIFIED BY archive_pass;
GRANT CONNECT, RESOURCE TO archive;
GRANT CREATE SEQUENCE TO archive;
ALTER USER archive DEFAULT TABLESPACE USERS;
ALTER USER archive QUOTA UNLIMITED ON USERS;

COMMIT;

-- Create tables in CRM schema (using fully qualified names)
CREATE TABLE crm.products (
    id NUMBER(10) PRIMARY KEY,
    name VARCHAR2(255) NOT NULL,
    sku VARCHAR2(50) NOT NULL,
    category VARCHAR2(100),
    price NUMBER(10,2),
    quantity NUMBER(10),
    created TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated TIMESTAMP DEFAULT SYSTIMESTAMP
);

CREATE SEQUENCE crm.seq_products START WITH 1 INCREMENT BY 1;

CREATE TABLE crm.customers (
    id NUMBER(10) PRIMARY KEY,
    first_name VARCHAR2(100),
    last_name VARCHAR2(100),
    email VARCHAR2(255),
    phone VARCHAR2(50),
    city VARCHAR2(100),
    country VARCHAR2(100),
    created TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated TIMESTAMP DEFAULT SYSTIMESTAMP
);

CREATE SEQUENCE crm.seq_customers START WITH 1 INCREMENT BY 1;

CREATE TABLE crm.orders (
    id NUMBER(10) PRIMARY KEY,
    product_id NUMBER(10),
    customer_name VARCHAR2(255),
    email VARCHAR2(255),
    status VARCHAR2(50),
    total_amount NUMBER(10,2),
    created TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated TIMESTAMP DEFAULT SYSTIMESTAMP
);

CREATE SEQUENCE crm.seq_orders START WITH 1 INCREMENT BY 1;

COMMIT;

-- Create tables in ARCHIVE schema (using fully qualified names)
CREATE TABLE archive.entity (
    id NUMBER(10) PRIMARY KEY,
    entity_type VARCHAR2(50),
    entity_name VARCHAR2(255),
    status VARCHAR2(50),
    created TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated TIMESTAMP DEFAULT SYSTIMESTAMP
);

CREATE SEQUENCE archive.seq_entity START WITH 1 INCREMENT BY 1;

COMMIT;

-- Grant permissions to ora2csv user
-- SELECT for data export (the main use case for ora2csv)
GRANT SELECT ON crm.products TO ora2csv;
GRANT SELECT ON crm.orders TO ora2csv;
GRANT SELECT ON crm.customers TO ora2csv;
GRANT SELECT ON archive.entity TO ora2csv;

-- INSERT for test data generation
GRANT INSERT ON crm.products TO ora2csv;
GRANT INSERT ON crm.orders TO ora2csv;
GRANT INSERT ON crm.customers TO ora2csv;
GRANT INSERT ON archive.entity TO ora2csv;

-- UPDATE for test data modification
GRANT UPDATE ON crm.products TO ora2csv;
GRANT UPDATE ON crm.orders TO ora2csv;
GRANT UPDATE ON crm.customers TO ora2csv;
GRANT UPDATE ON archive.entity TO ora2csv;

-- Grant sequence usage to ora2csv user
GRANT SELECT ON crm.seq_products TO ora2csv;
GRANT SELECT ON crm.seq_customers TO ora2csv;
GRANT SELECT ON crm.seq_orders TO ora2csv;
GRANT SELECT ON archive.seq_entity TO ora2csv;

-- Also grant READ for Oracle 21c
GRANT READ ON crm.products TO ora2csv;
GRANT READ ON crm.orders TO ora2csv;
GRANT READ ON crm.customers TO ora2csv;
GRANT READ ON archive.entity TO ora2csv;

COMMIT;

DBMS_OUTPUT.PUT_LINE('Schema initialization complete!');
EXIT;
