-- EasyStock schema. Amounts are NUMERIC, never float. New keys are UUID.
-- The migration role owns these tables. The application roles are created here
-- and granted in this file; row level security is applied in 000002.

CREATE OR REPLACE FUNCTION is_currency(c text) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
  SELECT c ~ '^[A-Z]{3}$'
$$;

CREATE OR REPLACE FUNCTION app_setting_uuid(setting text) RETURNS uuid
LANGUAGE sql STABLE AS $$
  SELECT NULLIF(current_setting(setting, true), '')::uuid
$$;

CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'easystock_app') THEN
    CREATE ROLE easystock_app LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'easystock_identity') THEN
    CREATE ROLE easystock_identity LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'easystock_job') THEN
    CREATE ROLE easystock_job LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
  END IF;
END $$;

CREATE TABLE subscription_plans (
  code text PRIMARY KEY,
  name text NOT NULL,
  monthly_price_usd numeric(20,4) NOT NULL CHECK (monthly_price_usd >= 0),
  max_stores int NOT NULL CHECK (max_stores > 0),
  max_active_members int NOT NULL CHECK (max_active_members > 0),
  allow_export boolean NOT NULL,
  allow_operational_roles boolean NOT NULL,
  allow_multistore boolean NOT NULL
);

INSERT INTO subscription_plans (
  code, name, monthly_price_usd, max_stores, max_active_members,
  allow_export, allow_operational_roles, allow_multistore
) VALUES
  ('essentiel', 'Essentiel', 5, 1, 2, false, false, false),
  ('pro', 'Pro', 10, 1, 5, true, true, false),
  ('business', 'Business', 20, 3, 15, true, true, true);

CREATE TABLE companies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_org_id text UNIQUE,
  name text NOT NULL,
  primary_currency char(3) NOT NULL DEFAULT 'USD',
  timezone text NOT NULL DEFAULT 'Africa/Lubumbashi',
  default_max_discount_rate numeric(8,6),
  default_max_credit numeric(20,4),
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT companies_currency_chk CHECK (is_currency(primary_currency)),
  CONSTRAINT companies_discount_chk CHECK (
    default_max_discount_rate IS NULL
    OR (default_max_discount_rate >= 0 AND default_max_discount_rate <= 1)
  ),
  CONSTRAINT companies_credit_chk CHECK (default_max_credit IS NULL OR default_max_credit >= 0)
);

CREATE TRIGGER companies_touch BEFORE UPDATE ON companies
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_user_id text UNIQUE,
  display_name text NOT NULL,
  phone text,
  email text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER users_touch BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE stores (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  name text NOT NULL,
  timezone text NOT NULL DEFAULT 'Africa/Lubumbashi',
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id)
);

CREATE INDEX stores_company_idx ON stores (company_id) WHERE archived_at IS NULL;
CREATE TRIGGER stores_touch BEFORE UPDATE ON stores
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE organization_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  user_id uuid NOT NULL REFERENCES users (id),
  org_role text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  clerk_membership_id text UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT memberships_role_chk CHECK (
    org_role IN ('owner', 'organization_admin', 'store_manager', 'cashier', 'stockkeeper', 'accountant')
  ),
  CONSTRAINT memberships_status_chk CHECK (status IN ('active', 'invited', 'revoked')),
  UNIQUE (company_id, user_id)
);

CREATE INDEX memberships_company_role_idx ON organization_memberships (company_id, org_role) WHERE status = 'active';
CREATE TRIGGER memberships_touch BEFORE UPDATE ON organization_memberships
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE store_assignments (
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  user_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (store_id, user_id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, user_id) REFERENCES organization_memberships (company_id, user_id)
);

CREATE INDEX store_assignments_user_idx ON store_assignments (company_id, user_id);

CREATE TABLE permission_overrides (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  user_id uuid NOT NULL,
  store_id uuid,
  permission text NOT NULL,
  effect text NOT NULL,
  max_discount_rate numeric(8,6),
  max_credit numeric(20,4),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT overrides_effect_chk CHECK (effect IN ('allow', 'deny')),
  CONSTRAINT overrides_discount_chk CHECK (
    max_discount_rate IS NULL OR (max_discount_rate >= 0 AND max_discount_rate <= 1)
  ),
  CONSTRAINT overrides_credit_chk CHECK (max_credit IS NULL OR max_credit >= 0),
  FOREIGN KEY (company_id, user_id) REFERENCES organization_memberships (company_id, user_id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id)
);

CREATE UNIQUE INDEX permission_overrides_uq
  ON permission_overrides (company_id, user_id, store_id, permission) NULLS NOT DISTINCT;

CREATE TABLE products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  sku text NOT NULL,
  name text NOT NULL,
  category text,
  base_unit text NOT NULL,
  barcode text,
  allow_fractional boolean NOT NULL DEFAULT false,
  alert_threshold numeric(20,6) NOT NULL DEFAULT 0 CHECK (alert_threshold >= 0),
  status text NOT NULL DEFAULT 'active',
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT products_status_chk CHECK (status IN ('active', 'archived')),
  UNIQUE (company_id, id),
  UNIQUE (company_id, sku)
);

CREATE UNIQUE INDEX products_barcode_uq ON products (company_id, barcode)
  WHERE barcode IS NOT NULL AND archived_at IS NULL;
CREATE INDEX products_sync_idx ON products (company_id, updated_at, id);
CREATE TRIGGER products_touch BEFORE UPDATE ON products
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE product_units (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  product_id uuid NOT NULL,
  name text NOT NULL,
  factor numeric(20,6) NOT NULL CHECK (factor > 0),
  is_base boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  UNIQUE (company_id, product_id, name),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id)
);

CREATE UNIQUE INDEX product_units_one_base ON product_units (product_id) WHERE is_base;

CREATE TABLE product_prices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  currency char(3) NOT NULL,
  unit_price numeric(20,4) NOT NULL CHECK (unit_price >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT prices_currency_chk CHECK (is_currency(currency)),
  UNIQUE (company_id, store_id, product_id, product_unit_id, currency),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE TRIGGER prices_touch BEFORE UPDATE ON product_prices
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE suppliers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  name text NOT NULL,
  phone text,
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id)
);

CREATE TRIGGER suppliers_touch BEFORE UPDATE ON suppliers
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE purchases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  supplier_id uuid NOT NULL,
  currency char(3) NOT NULL,
  total_cost numeric(20,4) NOT NULL CHECK (total_cost >= 0),
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT purchases_currency_chk CHECK (is_currency(currency)),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, supplier_id) REFERENCES suppliers (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE purchase_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  purchase_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  unit_cost numeric(20,4) NOT NULL CHECK (unit_cost >= 0),
  line_cost numeric(20,4) NOT NULL CHECK (line_cost >= 0),
  FOREIGN KEY (company_id, purchase_id) REFERENCES purchases (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE TABLE stock_balances (
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  product_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL DEFAULT 0,
  avg_cost numeric(20,4),
  cost_currency char(3),
  cost_complete boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, store_id, product_id),
  CONSTRAINT stock_balances_quantity_nonnegative CHECK (quantity >= 0),
  CONSTRAINT stock_cost_currency_chk CHECK (cost_currency IS NULL OR is_currency(cost_currency)),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id)
);

CREATE TRIGGER stock_balances_touch BEFORE UPDATE ON stock_balances
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE stock_movements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  product_id uuid NOT NULL,
  quantity_delta numeric(20,6) NOT NULL,
  movement_type text NOT NULL,
  reason text,
  ref_type text,
  ref_id uuid,
  actor_user_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT stock_movements_type_chk CHECK (
    movement_type IN ('receive', 'sale', 'refund', 'adjustment', 'opening')
  ),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id)
);

CREATE INDEX stock_movements_store_idx ON stock_movements (company_id, store_id, created_at DESC);

CREATE TABLE customers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  name text NOT NULL,
  phone text,
  notes text,
  credit_limit numeric(20,4),
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT customers_credit_chk CHECK (credit_limit IS NULL OR credit_limit >= 0),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id)
);

CREATE INDEX customers_store_idx ON customers (company_id, store_id);
CREATE TRIGGER customers_touch BEFORE UPDATE ON customers
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE cash_registers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  name text NOT NULL,
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id)
);

CREATE TABLE cash_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  register_id uuid NOT NULL,
  opened_by uuid NOT NULL,
  closed_by uuid,
  status text NOT NULL,
  opened_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz,
  UNIQUE (company_id, id),
  CONSTRAINT cash_sessions_status_chk CHECK (status IN ('open', 'closed')),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, register_id) REFERENCES cash_registers (company_id, id),
  FOREIGN KEY (company_id, opened_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE UNIQUE INDEX cash_sessions_one_open ON cash_sessions (company_id, register_id) WHERE status = 'open';

CREATE TABLE cash_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  session_id uuid NOT NULL,
  currency char(3) NOT NULL,
  direction text NOT NULL,
  kind text NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  ref_type text,
  ref_id uuid,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cash_tx_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT cash_tx_direction_chk CHECK (direction IN ('in', 'out')),
  CONSTRAINT cash_tx_kind_chk CHECK (
    kind IN ('opening', 'sale', 'debt_collection', 'refund', 'expense', 'supplier', 'adjustment')
  ),
  FOREIGN KEY (company_id, session_id) REFERENCES cash_sessions (company_id, id)
);

CREATE INDEX cash_tx_session_idx ON cash_transactions (session_id, currency);

CREATE TABLE cash_session_counts (
  company_id uuid NOT NULL,
  session_id uuid NOT NULL,
  currency char(3) NOT NULL,
  expected_amount numeric(20,4) NOT NULL,
  counted_amount numeric(20,4) NOT NULL,
  variance numeric(20,4) NOT NULL,
  variance_reason text,
  PRIMARY KEY (session_id, currency),
  CONSTRAINT cash_counts_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT cash_counts_variance_chk CHECK (variance = counted_amount - expected_amount),
  FOREIGN KEY (company_id, session_id) REFERENCES cash_sessions (company_id, id)
);

CREATE TABLE media_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  kind text NOT NULL,
  object_key text NOT NULL UNIQUE,
  content_type text NOT NULL,
  byte_size bigint,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id)
);

CREATE TABLE expenses (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  category text NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  paid_from_cash boolean NOT NULL,
  cash_session_id uuid,
  media_id uuid,
  note text,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT expenses_currency_chk CHECK (is_currency(currency)),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, cash_session_id) REFERENCES cash_sessions (company_id, id),
  FOREIGN KEY (company_id, media_id) REFERENCES media_objects (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE sale_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  client_draft_id text NOT NULL,
  payload jsonb NOT NULL,
  status text NOT NULL DEFAULT 'open',
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT sale_drafts_status_chk CHECK (status IN ('open', 'confirmed', 'abandoned')),
  UNIQUE (company_id, client_draft_id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id)
);

CREATE TRIGGER sale_drafts_touch BEFORE UPDATE ON sale_drafts
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE sales (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  customer_id uuid,
  operator_user_id uuid NOT NULL,
  currency char(3) NOT NULL,
  gross_amount numeric(20,4) NOT NULL,
  discount_amount numeric(20,4) NOT NULL DEFAULT 0,
  net_amount numeric(20,4) NOT NULL,
  paid_amount numeric(20,4) NOT NULL,
  due_amount numeric(20,4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT sales_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT sales_amounts_nonnegative CHECK (
    gross_amount >= 0 AND discount_amount >= 0 AND net_amount >= 0
    AND paid_amount >= 0 AND due_amount >= 0
  ),
  CONSTRAINT sales_net_chk CHECK (net_amount = gross_amount - discount_amount),
  CONSTRAINT sales_paid_chk CHECK (paid_amount + due_amount = net_amount),
  CONSTRAINT sales_discount_chk CHECK (discount_amount <= gross_amount),
  UNIQUE (company_id, id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, customer_id) REFERENCES customers (company_id, id),
  FOREIGN KEY (company_id, operator_user_id) REFERENCES organization_memberships (company_id, user_id)
);

CREATE INDEX sales_store_time_idx ON sales (company_id, store_id, created_at DESC, id);
CREATE INDEX sales_currency_time_idx ON sales (company_id, currency, created_at);

CREATE TABLE sale_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  sale_id uuid NOT NULL,
  product_id uuid NOT NULL,
  product_unit_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  unit_price numeric(20,4) NOT NULL CHECK (unit_price >= 0),
  line_gross numeric(20,4) NOT NULL CHECK (line_gross >= 0),
  line_net numeric(20,4) NOT NULL CHECK (line_net >= 0),
  unit_cost numeric(20,4),
  cost_complete boolean NOT NULL,
  UNIQUE (company_id, id),
  FOREIGN KEY (company_id, sale_id) REFERENCES sales (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id),
  FOREIGN KEY (company_id, product_unit_id) REFERENCES product_units (company_id, id)
);

CREATE TABLE debts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  customer_id uuid NOT NULL,
  sale_id uuid NOT NULL,
  currency char(3) NOT NULL,
  original_amount numeric(20,4) NOT NULL CHECK (original_amount >= 0),
  collected_amount numeric(20,4) NOT NULL DEFAULT 0 CHECK (collected_amount >= 0),
  offset_amount numeric(20,4) NOT NULL DEFAULT 0 CHECK (offset_amount >= 0),
  due_amount numeric(20,4) NOT NULL CHECK (due_amount >= 0),
  due_on date,
  status text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT debts_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT debts_balance_chk CHECK (collected_amount + offset_amount + due_amount = original_amount),
  CONSTRAINT debts_status_chk CHECK (status IN ('open', 'partial', 'settled')),
  UNIQUE (company_id, id),
  UNIQUE (sale_id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, customer_id) REFERENCES customers (company_id, id),
  FOREIGN KEY (company_id, sale_id) REFERENCES sales (company_id, id)
);

CREATE INDEX debts_store_status_idx ON debts (company_id, store_id, status, due_on);
CREATE TRIGGER debts_touch BEFORE UPDATE ON debts
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  sale_id uuid,
  debt_id uuid,
  method text NOT NULL,
  currency char(3) NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  external_reference text,
  cash_session_id uuid,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payments_method_chk CHECK (method IN ('cash', 'external')),
  CONSTRAINT payments_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT payments_link_chk CHECK (sale_id IS NOT NULL OR debt_id IS NOT NULL),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, sale_id) REFERENCES sales (company_id, id),
  FOREIGN KEY (company_id, debt_id) REFERENCES debts (company_id, id),
  FOREIGN KEY (company_id, cash_session_id) REFERENCES cash_sessions (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE INDEX payments_sale_idx ON payments (company_id, sale_id);

CREATE TABLE refunds (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  sale_id uuid NOT NULL,
  currency char(3) NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount >= 0),
  cash_amount numeric(20,4) NOT NULL CHECK (cash_amount >= 0),
  debt_amount numeric(20,4) NOT NULL CHECK (debt_amount >= 0),
  reason text NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT refunds_split_chk CHECK (amount = cash_amount + debt_amount),
  CONSTRAINT refunds_currency_chk CHECK (is_currency(currency)),
  UNIQUE (company_id, id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, sale_id) REFERENCES sales (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE refund_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  refund_id uuid NOT NULL,
  sale_item_id uuid NOT NULL,
  quantity numeric(20,6) NOT NULL CHECK (quantity > 0),
  base_quantity numeric(20,6) NOT NULL CHECK (base_quantity > 0),
  restock boolean NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount >= 0),
  FOREIGN KEY (company_id, refund_id) REFERENCES refunds (company_id, id),
  FOREIGN KEY (company_id, sale_item_id) REFERENCES sale_items (company_id, id)
);

CREATE INDEX refund_items_sale_item_idx ON refund_items (sale_item_id);

CREATE TABLE debt_payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  debt_id uuid NOT NULL,
  kind text NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  payment_id uuid,
  refund_id uuid,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT debt_payments_kind_chk CHECK (kind IN ('collection', 'refund_offset')),
  FOREIGN KEY (company_id, debt_id) REFERENCES debts (company_id, id),
  FOREIGN KEY (company_id, refund_id) REFERENCES refunds (company_id, id)
);

CREATE TABLE supplier_payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  supplier_id uuid NOT NULL,
  purchase_id uuid,
  currency char(3) NOT NULL,
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  method text NOT NULL,
  cash_session_id uuid,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT supplier_payments_method_chk CHECK (method IN ('cash', 'external')),
  CONSTRAINT supplier_payments_currency_chk CHECK (is_currency(currency)),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, supplier_id) REFERENCES suppliers (company_id, id),
  FOREIGN KEY (company_id, purchase_id) REFERENCES purchases (company_id, id),
  FOREIGN KEY (company_id, cash_session_id) REFERENCES cash_sessions (company_id, id)
);

CREATE TABLE inventory_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  store_id uuid NOT NULL,
  status text NOT NULL,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  UNIQUE (company_id, id),
  CONSTRAINT inventory_status_chk CHECK (status IN ('draft', 'completed', 'cancelled')),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (company_id, created_by) REFERENCES organization_memberships (company_id, user_id)
);

CREATE TABLE inventory_counts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL,
  inventory_id uuid NOT NULL,
  product_id uuid NOT NULL,
  counted_quantity numeric(20,6) NOT NULL CHECK (counted_quantity >= 0),
  reason text,
  UNIQUE (inventory_id, product_id),
  FOREIGN KEY (company_id, inventory_id) REFERENCES inventory_sessions (company_id, id),
  FOREIGN KEY (company_id, product_id) REFERENCES products (company_id, id)
);

CREATE TABLE subscription_snapshots (
  company_id uuid PRIMARY KEY REFERENCES companies (id),
  plan_code text NOT NULL REFERENCES subscription_plans (code),
  status text NOT NULL,
  source text NOT NULL,
  trial_ends_at timestamptz,
  current_period_end timestamptz,
  grace_ends_at timestamptz,
  external_id text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT subscription_status_chk CHECK (status IN ('trialing', 'active', 'grace', 'expired', 'canceled')),
  CONSTRAINT subscription_source_chk CHECK (source IN ('trial', 'clerk', 'local'))
);

CREATE TRIGGER subscription_snapshots_touch BEFORE UPDATE ON subscription_snapshots
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE local_subscription_payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  plan_code text NOT NULL REFERENCES subscription_plans (code),
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  external_reference text NOT NULL,
  status text NOT NULL,
  confirmed_by uuid,
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT local_payments_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT local_payments_status_chk CHECK (status IN ('pending', 'confirmed')),
  UNIQUE (company_id, external_reference)
);

CREATE TABLE entitlement_grants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  source text NOT NULL,
  plan_code text NOT NULL REFERENCES subscription_plans (code),
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  local_payment_id uuid UNIQUE,
  created_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT grants_source_chk CHECK (source IN ('local', 'clerk', 'trial')),
  CONSTRAINT grants_period_chk CHECK (ends_at > starts_at),
  FOREIGN KEY (local_payment_id) REFERENCES local_subscription_payments (id)
);

CREATE INDEX entitlement_grants_company_idx ON entitlement_grants (company_id, ends_at DESC);

CREATE TABLE webhook_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider text NOT NULL,
  event_id text NOT NULL,
  event_type text NOT NULL,
  occurred_at timestamptz NOT NULL,
  payload_hash text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, event_id),
  CONSTRAINT webhook_status_chk CHECK (status IN ('applied', 'ignored_duplicate', 'ignored_stale', 'ignored_unknown'))
);

CREATE TABLE projection_versions (
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (entity_type, entity_id)
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  store_id uuid,
  actor_type text NOT NULL,
  actor_user_id uuid,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT audit_actor_chk CHECK (actor_type IN ('user', 'operator', 'system'))
);

CREATE INDEX audit_logs_company_idx ON audit_logs (company_id, created_at DESC);

CREATE TABLE idempotency_keys (
  company_id uuid NOT NULL,
  command_type text NOT NULL,
  idempotency_key text NOT NULL,
  request_hash text NOT NULL,
  status text NOT NULL,
  resource_id uuid,
  response_json jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, command_type, idempotency_key),
  CONSTRAINT idempotency_status_chk CHECK (status IN ('processing', 'completed'))
);

CREATE TABLE legacy_id_map (
  entity_type text NOT NULL,
  legacy_id text NOT NULL,
  new_id uuid NOT NULL,
  company_id uuid,
  PRIMARY KEY (entity_type, legacy_id)
);

CREATE TABLE company_exchange_rates (
  company_id uuid NOT NULL REFERENCES companies (id),
  from_currency char(3) NOT NULL,
  to_currency char(3) NOT NULL,
  rate numeric(20,8) NOT NULL CHECK (rate > 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (company_id, from_currency, to_currency),
  CONSTRAINT exchange_pair_chk CHECK (from_currency <> to_currency AND is_currency(from_currency) AND is_currency(to_currency))
);

CREATE OR REPLACE FUNCTION reject_sale_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'confirmed sale is immutable' USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER sales_immutable BEFORE UPDATE OR DELETE ON sales
FOR EACH ROW EXECUTE FUNCTION reject_sale_mutation();

CREATE OR REPLACE FUNCTION reject_closed_cash_session_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'closed' THEN
    RAISE EXCEPTION 'closed cash session is immutable' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER cash_sessions_immutable BEFORE UPDATE OR DELETE ON cash_sessions
FOR EACH ROW EXECUTE FUNCTION reject_closed_cash_session_update();

CREATE OR REPLACE FUNCTION reject_movement_on_closed_session() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  session_status text;
BEGIN
  SELECT status INTO session_status FROM cash_sessions WHERE id = NEW.session_id;
  IF session_status IS DISTINCT FROM 'open' THEN
    RAISE EXCEPTION 'closed cash session is immutable' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER cash_tx_open_session BEFORE INSERT ON cash_transactions
FOR EACH ROW EXECUTE FUNCTION reject_movement_on_closed_session();

CREATE OR REPLACE FUNCTION prevent_last_owner_removal() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  remaining int;
BEGIN
  IF OLD.org_role = 'owner' AND OLD.status = 'active' AND (
    TG_OP = 'DELETE' OR NEW.org_role <> 'owner' OR NEW.status <> 'active'
  ) THEN
    SELECT count(*) INTO remaining
    FROM organization_memberships
    WHERE company_id = OLD.company_id
      AND org_role = 'owner'
      AND status = 'active'
      AND user_id <> OLD.user_id;
    IF remaining = 0 THEN
      RAISE EXCEPTION 'cannot remove the last owner' USING ERRCODE = '23514';
    END IF;
  END IF;
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER memberships_last_owner BEFORE UPDATE OR DELETE ON organization_memberships
FOR EACH ROW EXECUTE FUNCTION prevent_last_owner_removal();

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO easystock_app, easystock_identity, easystock_job;

GRANT SELECT ON subscription_plans TO easystock_app, easystock_identity, easystock_job;

GRANT SELECT ON users TO easystock_app, easystock_job;
GRANT SELECT, INSERT, UPDATE ON users TO easystock_identity;

GRANT SELECT, INSERT, UPDATE ON
  companies, stores, organization_memberships, store_assignments, permission_overrides,
  products, product_units, product_prices, suppliers, purchases, stock_balances,
  customers, cash_registers, cash_sessions, media_objects, expenses, sale_drafts,
  debts, inventory_sessions, subscription_snapshots, local_subscription_payments,
  entitlement_grants, idempotency_keys, company_exchange_rates
TO easystock_app;

GRANT SELECT, INSERT ON
  purchase_items, stock_movements, sales, sale_items, payments, refunds, refund_items,
  debt_payments, supplier_payments, cash_transactions, cash_session_counts,
  inventory_counts, audit_logs
TO easystock_app;

GRANT SELECT, INSERT, UPDATE ON
  companies, users, organization_memberships, subscription_snapshots, webhook_events, projection_versions
TO easystock_identity;

GRANT SELECT ON companies TO easystock_job;
GRANT SELECT, UPDATE ON subscription_snapshots TO easystock_job;

GRANT EXECUTE ON FUNCTION app_setting_uuid(text) TO easystock_app, easystock_identity, easystock_job;
