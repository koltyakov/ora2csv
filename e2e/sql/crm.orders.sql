-- Export orders with incremental sync based on updated timestamp
SELECT
    o.id,
    o.product_id,
    o.customer_name,
    o.email,
    o.status,
    o.total_amount,
    p.name as product_name,
    p.sku as product_sku,
    TO_CHAR(o.created, 'YYYY-MM-DD"T"HH24:MI:SS') as created,
    TO_CHAR(o.updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM crm.orders o
LEFT JOIN crm.products p ON o.product_id = p.id
WHERE o.updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND o.updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY o.updated ASC