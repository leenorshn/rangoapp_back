-- Tenant isolation. The application role is not the table owner and cannot bypass RLS.
-- set_config(..., true) in each transaction is the only tenant context; it does not
-- survive COMMIT and therefore cannot leak across the connection pool.

ALTER TABLE subscription_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_plans FORCE ROW LEVEL SECURITY;
CREATE POLICY plans_read ON subscription_plans
  FOR SELECT TO easystock_app, easystock_identity, easystock_job
  USING (true);

ALTER TABLE companies ENABLE ROW LEVEL SECURITY;
ALTER TABLE companies FORCE ROW LEVEL SECURITY;
CREATE POLICY companies_app ON companies
  FOR ALL TO easystock_app
  USING (id = app_setting_uuid('app.company_id'))
  WITH CHECK (id = app_setting_uuid('app.company_id'));
CREATE POLICY companies_identity ON companies
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);
CREATE POLICY companies_job_read ON companies
  FOR SELECT TO easystock_job
  USING (true);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY users_app ON users
  FOR SELECT TO easystock_app, easystock_job
  USING (
    id = app_setting_uuid('app.user_id')
    OR EXISTS (
      SELECT 1 FROM organization_memberships m
      WHERE m.user_id = users.id
        AND m.company_id = app_setting_uuid('app.company_id')
    )
  );
CREATE POLICY users_identity ON users
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);

DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'stores', 'organization_memberships', 'store_assignments', 'permission_overrides',
    'products', 'product_units', 'product_prices', 'suppliers', 'purchases', 'purchase_items',
    'stock_balances', 'stock_movements', 'customers', 'cash_registers', 'cash_sessions',
    'cash_transactions', 'cash_session_counts', 'media_objects', 'expenses', 'sale_drafts',
    'sales', 'sale_items', 'debts', 'payments', 'refunds', 'refund_items', 'debt_payments',
    'supplier_payments', 'inventory_sessions', 'inventory_counts', 'subscription_snapshots',
    'local_subscription_payments', 'entitlement_grants', 'idempotency_keys', 'audit_logs',
    'company_exchange_rates'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I ON %I FOR ALL TO easystock_app USING (company_id = app_setting_uuid(''app.company_id'')) WITH CHECK (company_id = app_setting_uuid(''app.company_id''))',
      t || '_tenant', t
    );
  END LOOP;
END $$;

CREATE POLICY subscription_snapshots_job ON subscription_snapshots
  FOR ALL TO easystock_job
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

CREATE POLICY subscription_snapshots_identity ON subscription_snapshots
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);

CREATE POLICY memberships_identity ON organization_memberships
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);

ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_events FORCE ROW LEVEL SECURITY;
CREATE POLICY webhook_events_identity ON webhook_events
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);

ALTER TABLE projection_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE projection_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY projection_versions_identity ON projection_versions
  FOR ALL TO easystock_identity
  USING (true) WITH CHECK (true);

-- Import runs as the migration owner. The application role has no policy and no grant.
ALTER TABLE legacy_id_map ENABLE ROW LEVEL SECURITY;

GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO easystock_app, easystock_identity, easystock_job;
