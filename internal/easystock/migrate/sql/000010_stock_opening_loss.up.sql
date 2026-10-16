-- An opening is the stock already in the shop. It is not a supplier receipt.
-- A loss keeps the exit value at the average cost of that moment.
-- Older movements leave the new columns null.

ALTER TABLE stock_movements
  ADD COLUMN origin_factor numeric(20,6),
  ADD COLUMN exit_value numeric(20,4),
  ADD COLUMN exit_currency char(3),
  ADD CONSTRAINT stock_movements_origin_factor_chk CHECK (origin_factor IS NULL OR origin_factor > 0),
  ADD CONSTRAINT stock_movements_exit_value_chk CHECK (exit_value IS NULL OR exit_value >= 0),
  ADD CONSTRAINT stock_movements_exit_currency_chk CHECK (exit_currency IS NULL OR is_currency(exit_currency)),
  ADD CONSTRAINT stock_movements_exit_pair_chk CHECK (
    (exit_value IS NULL AND exit_currency IS NULL)
    OR (exit_value IS NOT NULL AND exit_currency IS NOT NULL)
  );

CREATE TABLE stock_openings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  note text,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT stock_openings_note_len CHECK (note IS NULL OR char_length(note) <= 500),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE stock_opening_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  opening_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  factor numeric(20,6) NOT NULL CHECK (factor > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  unit_cost numeric(20,4),
  currency char(3),
  line_cost numeric(20,4),
  UNIQUE (company_id, id),
  UNIQUE (opening_id, product_unit_id),
  CONSTRAINT stock_opening_lines_cost_chk CHECK (unit_cost IS NULL OR unit_cost >= 0),
  CONSTRAINT stock_opening_lines_line_cost_chk CHECK (line_cost IS NULL OR line_cost >= 0),
  CONSTRAINT stock_opening_lines_currency_chk CHECK (currency IS NULL OR is_currency(currency)),
  CONSTRAINT stock_opening_lines_cost_pair_chk CHECK (
    (unit_cost IS NULL AND currency IS NULL AND line_cost IS NULL)
    OR (unit_cost IS NOT NULL AND currency IS NOT NULL AND line_cost IS NOT NULL)
  ),
  FOREIGN KEY (company_id, opening_id) REFERENCES stock_openings (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE INDEX stock_opening_lines_opening_idx ON stock_opening_lines (opening_id);

-- One opening per product and store. Later receipts stay allowed.
CREATE TABLE stock_product_openings (
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  product_id uuid NOT NULL,
  opening_id uuid NOT NULL,
  CONSTRAINT stock_product_openings_once PRIMARY KEY (company_id, store_id, product_id),
  FOREIGN KEY (company_id, opening_id) REFERENCES stock_openings (company_id, id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id)
);

ALTER TABLE stock_openings ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_openings FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_openings_tenant ON stock_openings
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_opening_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_opening_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_opening_lines_tenant ON stock_opening_lines
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_product_openings ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_product_openings FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_product_openings_tenant ON stock_product_openings
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

GRANT SELECT, INSERT ON stock_openings, stock_opening_lines, stock_product_openings TO easystock_app;
