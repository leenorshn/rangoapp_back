DROP POLICY IF EXISTS stock_product_openings_tenant ON stock_product_openings;
DROP TABLE IF EXISTS stock_product_openings;

DROP POLICY IF EXISTS stock_opening_lines_tenant ON stock_opening_lines;
DROP TABLE IF EXISTS stock_opening_lines;

DROP POLICY IF EXISTS stock_openings_tenant ON stock_openings;
DROP TABLE IF EXISTS stock_openings;

ALTER TABLE stock_movements
  DROP CONSTRAINT IF EXISTS stock_movements_exit_pair_chk,
  DROP CONSTRAINT IF EXISTS stock_movements_exit_currency_chk,
  DROP CONSTRAINT IF EXISTS stock_movements_exit_value_chk,
  DROP CONSTRAINT IF EXISTS stock_movements_origin_factor_chk,
  DROP COLUMN IF EXISTS exit_currency,
  DROP COLUMN IF EXISTS exit_value,
  DROP COLUMN IF EXISTS origin_factor;
