-- name: ListCatalogChanges :many
SELECT company_id, id, sku, name, description, category, category_id, barcode, base_unit,
       allow_fractional, alert_threshold, status, sync_at AS updated_at, archived,
       density_kg_per_liter, density_reference, density_updated_at, density_updated_by,
       cross_dimension_conversion_enabled
FROM (
  SELECT p.company_id, p.id, p.sku, p.name, p.description, p.category, p.category_id, p.barcode,
         p.base_unit, p.allow_fractional, p.alert_threshold, p.status,
         CASE
           WHEN sqlc.narg(store_id)::uuid IS NULL THEN p.updated_at
           ELSE GREATEST(
             p.updated_at,
             COALESCE((
               SELECT max(pp.updated_at)
               FROM product_prices pp
               WHERE pp.product_id = p.id
                 AND pp.store_id = sqlc.narg(store_id)::uuid
             ), p.updated_at)
           )
         END::timestamptz AS sync_at,
         (p.archived_at IS NOT NULL)::bool AS archived,
         p.density_kg_per_liter::text AS density_kg_per_liter,
         p.density_reference,
         p.density_updated_at,
         p.density_updated_by,
         p.cross_dimension_conversion_enabled
  FROM products p
) page
WHERE (sync_at, id) > (sqlc.arg(after)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY sync_at, id
LIMIT sqlc.arg(row_limit);

-- name: ListUnitsForProducts :many
SELECT id, product_id, name, factor, dimension, barcode, is_base, active,
       allow_purchase, allow_sale, allow_fractional
FROM product_units
WHERE product_id = ANY(sqlc.arg(product_ids)::uuid[])
ORDER BY is_base DESC, name, id;

-- name: ListPricesForStore :many
SELECT id, store_id, product_id, product_unit_id, currency, unit_price
FROM product_prices
WHERE store_id = sqlc.arg(store_id)
  AND archived_at IS NULL
  AND product_id = ANY(sqlc.arg(product_ids)::uuid[])
ORDER BY product_unit_id, currency;
