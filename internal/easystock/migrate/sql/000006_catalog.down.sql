DROP TRIGGER IF EXISTS product_units_touch_product ON product_units;
DROP TRIGGER IF EXISTS product_units_touch ON product_units;
DROP FUNCTION IF EXISTS catalog_touch_product();

DROP INDEX IF EXISTS product_prices_sync_idx;
ALTER TABLE product_prices DROP COLUMN IF EXISTS archived_at;

DROP INDEX IF EXISTS product_units_product_idx;
DROP INDEX IF EXISTS product_units_barcode_uq;
ALTER TABLE product_units DROP CONSTRAINT IF EXISTS product_units_base_active_chk;
ALTER TABLE product_units DROP CONSTRAINT IF EXISTS product_units_base_factor_chk;
ALTER TABLE product_units DROP COLUMN IF EXISTS updated_at;
ALTER TABLE product_units DROP COLUMN IF EXISTS active;
ALTER TABLE product_units DROP COLUMN IF EXISTS barcode;

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_category_fk;
ALTER TABLE products DROP COLUMN IF EXISTS category_id;
ALTER TABLE products DROP COLUMN IF EXISTS description;

DROP TABLE IF EXISTS product_categories;
