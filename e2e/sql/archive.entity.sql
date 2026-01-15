-- Export archive.entity with incremental sync
SELECT
    id,
    entity_type,
    entity_name,
    status,
    TO_CHAR(created, 'YYYY-MM-DD"T"HH24:MI:SS') as created,
    TO_CHAR(updated, 'YYYY-MM-DD"T"HH24:MI:SS') as updated
FROM archive.entity
WHERE updated >= TO_DATE(:startDate, 'YYYY-MM-DD"T"HH24:MI:SS')
  AND updated < TO_DATE(:tillDate, 'YYYY-MM-DD"T"HH24:MI:SS')
ORDER BY updated ASC