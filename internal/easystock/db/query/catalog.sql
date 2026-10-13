-- name: ListCatalogChanges :many
SELECT id, sku, name, base_unit, status, updated_at, (archived_at IS NOT NULL)::bool AS archived
FROM products
WHERE (updated_at, id) > (sqlc.arg(after)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY updated_at, id
LIMIT sqlc.arg(row_limit);
