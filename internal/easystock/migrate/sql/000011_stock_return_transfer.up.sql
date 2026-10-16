-- Supplier returns and linked store transfers.
-- In-transit quantity stays on the transfer document, not on a third balance.
-- Exit value is the average cost at dispatch or return. Older rows stay null.

CREATE TABLE stock_supplier_returns (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  supplier_id uuid,
  reference text,
  reason text NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT stock_supplier_returns_reference_len CHECK (reference IS NULL OR char_length(reference) <= 80),
  CONSTRAINT stock_supplier_returns_reason_len CHECK (char_length(btrim(reason)) BETWEEN 1 AND 500),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, supplier_id) REFERENCES suppliers (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE stock_supplier_return_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  return_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  factor numeric(20,6) NOT NULL CHECK (factor > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  exit_value numeric(20,4),
  exit_currency char(3),
  UNIQUE (company_id, id),
  UNIQUE (return_id, product_unit_id),
  CONSTRAINT stock_supplier_return_lines_exit_value_chk CHECK (exit_value IS NULL OR exit_value >= 0),
  CONSTRAINT stock_supplier_return_lines_exit_currency_chk CHECK (exit_currency IS NULL OR is_currency(exit_currency)),
  CONSTRAINT stock_supplier_return_lines_exit_pair_chk CHECK (
    (exit_value IS NULL AND exit_currency IS NULL)
    OR (exit_value IS NOT NULL AND exit_currency IS NOT NULL)
  ),
  FOREIGN KEY (company_id, return_id) REFERENCES stock_supplier_returns (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE INDEX stock_supplier_return_lines_return_idx ON stock_supplier_return_lines (return_id);

CREATE TABLE stock_transfers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  source_store_id uuid NOT NULL,
  destination_store_id uuid NOT NULL,
  status text NOT NULL,
  reference text,
  note text,
  version integer NOT NULL DEFAULT 1 CHECK (version > 0),
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  dispatched_by uuid,
  dispatched_at timestamptz,
  received_by uuid,
  received_at timestamptz,
  cancelled_by uuid,
  cancelled_at timestamptz,
  UNIQUE (company_id, id),
  CONSTRAINT stock_transfers_status_chk CHECK (status IN ('draft', 'in_transit', 'received', 'cancelled')),
  CONSTRAINT stock_transfers_distinct_store_chk CHECK (source_store_id <> destination_store_id),
  CONSTRAINT stock_transfers_reference_len CHECK (reference IS NULL OR char_length(reference) <= 80),
  CONSTRAINT stock_transfers_note_len CHECK (note IS NULL OR char_length(note) <= 500),
  FOREIGN KEY (company_id, source_store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, destination_store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id),
  FOREIGN KEY (company_id, dispatched_by) REFERENCES organization_memberships (company_id, user_id),
  FOREIGN KEY (company_id, received_by) REFERENCES organization_memberships (company_id, user_id),
  FOREIGN KEY (company_id, cancelled_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE INDEX stock_transfers_source_idx ON stock_transfers (company_id, source_store_id, status);
CREATE INDEX stock_transfers_destination_idx ON stock_transfers (company_id, destination_store_id, status);

CREATE TABLE stock_transfer_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  transfer_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  factor numeric(20,6) NOT NULL CHECK (factor > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  exit_unit_cost numeric(20,4),
  exit_value numeric(20,4),
  exit_currency char(3),
  UNIQUE (company_id, id),
  UNIQUE (transfer_id, product_unit_id),
  CONSTRAINT stock_transfer_lines_exit_cost_chk CHECK (exit_unit_cost IS NULL OR exit_unit_cost >= 0),
  CONSTRAINT stock_transfer_lines_exit_value_chk CHECK (exit_value IS NULL OR exit_value >= 0),
  CONSTRAINT stock_transfer_lines_exit_currency_chk CHECK (exit_currency IS NULL OR is_currency(exit_currency)),
  CONSTRAINT stock_transfer_lines_exit_pair_chk CHECK (
    (exit_unit_cost IS NULL AND exit_value IS NULL AND exit_currency IS NULL)
    OR (exit_unit_cost IS NOT NULL AND exit_value IS NOT NULL AND exit_currency IS NOT NULL)
  ),
  FOREIGN KEY (company_id, transfer_id) REFERENCES stock_transfers (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE INDEX stock_transfer_lines_transfer_idx ON stock_transfer_lines (transfer_id);

CREATE TABLE stock_transfer_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  transfer_id uuid NOT NULL,
  from_status text,
  to_status text NOT NULL,
  actor_user_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT stock_transfer_events_from_chk CHECK (
    from_status IS NULL OR from_status IN ('draft', 'in_transit', 'received', 'cancelled')
  ),
  CONSTRAINT stock_transfer_events_to_chk CHECK (to_status IN ('draft', 'in_transit', 'received', 'cancelled')),
  FOREIGN KEY (company_id, transfer_id) REFERENCES stock_transfers (company_id, id),
  FOREIGN KEY (company_id, actor_user_id) REFERENCES organization_memberships (company_id, user_id)
);

CREATE INDEX stock_transfer_events_transfer_idx ON stock_transfer_events (transfer_id, created_at, id);

ALTER TABLE stock_supplier_returns ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_supplier_returns FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_supplier_returns_tenant ON stock_supplier_returns
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_supplier_return_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_supplier_return_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_supplier_return_lines_tenant ON stock_supplier_return_lines
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_transfers FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_transfers_tenant ON stock_transfers
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_transfer_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_transfer_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_transfer_lines_tenant ON stock_transfer_lines
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

ALTER TABLE stock_transfer_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_transfer_events FORCE ROW LEVEL SECURITY;
CREATE POLICY stock_transfer_events_tenant ON stock_transfer_events
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

GRANT SELECT, INSERT ON stock_supplier_returns, stock_supplier_return_lines, stock_transfer_events TO easystock_app;
GRANT SELECT, INSERT, UPDATE ON stock_transfers, stock_transfer_lines TO easystock_app;
