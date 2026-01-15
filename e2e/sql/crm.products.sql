-- Export products with incremental sync based on updated timestamp
SELECT
    id,
    name,
    sku,
    category,
    price,
    quantity,
    TO_CHAR(created, 'YYYY-MM-DD"T"HH24:MI:SS') as created,
    TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM crm.products
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY updated ASC