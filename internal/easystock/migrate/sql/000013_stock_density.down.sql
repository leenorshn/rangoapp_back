ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_density_positive;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS density_kg_per_liter;

ALTER TABLE product_units DROP CONSTRAINT IF EXISTS product_units_dimension_chk;
ALTER TABLE product_units DROP COLUMN IF EXISTS dimension;

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_density_updated_by_fkey;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_density_required;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_density_reference_len;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_density_positive;
ALTER TABLE products DROP COLUMN IF EXISTS cross_dimension_conversion_enabled;
ALTER TABLE products DROP COLUMN IF EXISTS density_updated_by;
ALTER TABLE products DROP COLUMN IF EXISTS density_updated_at;
ALTER TABLE products DROP COLUMN IF EXISTS density_reference;
ALTER TABLE products DROP COLUMN IF EXISTS density_kg_per_liter;
