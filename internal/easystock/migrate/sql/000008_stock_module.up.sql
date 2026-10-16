-- Purchase and sale rights on existing packagings, stock receipts without a
-- supplier, and a counted inventory that posts audited adjustments.
-- Stock stays one balance per product, in the base unit. Existing quantities
-- are not rewritten.

ALTER TABLE product_units
  ADD COLUMN allow_purchase boolean NOT NULL DEFAULT true,
  ADD COLUMN allow_sale boolean NOT NULL DEFAULT true,
  ADD COLUMN allow_fractional boolean NOT NULL DEFAULT false;

UPDATE product_units u
SET allow_fractional = p.allow_fractional
FROM products p
WHERE p.company_id = u.company_id
  AND p.id = u.product_id;

ALTER TABLE purchases
  ALTER COLUMN supplier_id DROP NOT NULL,
  ADD COLUMN reference text,
  ADD COLUMN note text;

ALTER TABLE purchases
  ADD CONSTRAINT purchases_reference_len CHECK (reference IS NULL OR char_length(reference) <= 80),
  ADD CONSTRAINT purchases_note_len CHECK (note IS NULL OR char_length(note) <= 500);

ALTER TABLE inventory_sessions
  DROP CONSTRAINT inventory_status_chk;

ALTER TABLE inventory_sessions
  ADD CONSTRAINT inventory_status_chk CHECK (status IN ('draft', 'counting', 'completed', 'cancelled')),
  ADD COLUMN note text,
  ADD COLUMN completed_by uuid,
  ADD CONSTRAINT inventory_sessions_note_len CHECK (note IS NULL OR char_length(note) <= 500),
  ADD CONSTRAINT inventory_sessions_completed_by_fk
    FOREIGN KEY (company_id, completed_by) REFERENCES organization_memberships (company_id, user_id);

CREATE UNIQUE INDEX inventory_sessions_one_open
  ON inventory_sessions (company_id, store_id)
  WHERE status IN ('draft', 'counting');

CREATE TABLE inventory_count_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  inventory_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  counted_quantity numeric(20,6) NOT NULL CHECK (counted_quantity >= 0),
  factor numeric(20,6) NOT NULL CHECK (factor > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity >= 0),
  reason text,
  balance_at_count numeric(20,6) NOT NULL CHECK (balance_at_count >= 0),
  counted_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  UNIQUE (inventory_id, product_unit_id),
  CONSTRAINT inventory_count_lines_reason_len CHECK (reason IS NULL OR char_length(reason) <= 500),
  FOREIGN KEY (company_id, inventory_id) REFERENCES inventory_sessions (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE INDEX inventory_count_lines_product_idx
  ON inventory_count_lines (inventory_id, product_id);

CREATE INDEX inventory_count_lines_unit_idx
  ON inventory_count_lines (company_id, product_unit_id);

ALTER TABLE inventory_counts
  ADD COLUMN theoretical_quantity numeric(20,6),
  ADD COLUMN variance_quantity numeric(20,6),
  ADD COLUMN variance_value numeric(20,4),
  ADD COLUMN variance_currency char(3),
  ADD CONSTRAINT inventory_counts_variance_currency_chk CHECK (
    variance_currency IS NULL OR is_currency(variance_currency)
  );

ALTER TABLE stock_movements
  DROP CONSTRAINT stock_movements_type_chk;

ALTER TABLE stock_movements
  ADD CONSTRAINT stock_movements_type_chk CHECK (
    movement_type IN (
      'receive', 'sale', 'refund', 'adjustment', 'opening',
      'purchase', 'receipt', 'sale_return', 'purchase_return',
      'inventory_adjustment', 'loss', 'transfer_in', 'transfer_out'
    )
  ),
  ADD COLUMN origin_unit_id uuid,
  ADD COLUMN origin_unit_name text,
  ADD COLUMN origin_quantity numeric(20,6),
  ADD COLUMN note text;

CREATE INDEX stock_movements_product_idx
  ON stock_movements (company_id, store_id, product_id, created_at DESC, id DESC);

ALTER TABLE inventory_count_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE inventory_count_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY inventory_count_lines_tenant ON inventory_count_lines
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

GRANT SELECT, INSERT, UPDATE ON inventory_count_lines TO easystock_app;
