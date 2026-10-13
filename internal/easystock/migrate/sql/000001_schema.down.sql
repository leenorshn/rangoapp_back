DROP TABLE IF EXISTS
  legacy_id_map, company_exchange_rates, idempotency_keys, audit_logs, projection_versions,
  webhook_events, entitlement_grants, local_subscription_payments, subscription_snapshots,
  inventory_counts, inventory_sessions, supplier_payments, debt_payments, refund_items, refunds,
  payments, debts, sale_items, sales, sale_drafts, expenses, media_objects, cash_session_counts,
  cash_transactions, cash_sessions, cash_registers, customers, stock_movements, stock_balances,
  purchase_items, purchases, suppliers, product_prices, product_units, products,
  permission_overrides, store_assignments, organization_memberships, stores, users, companies,
  subscription_plans
CASCADE;

DROP FUNCTION IF EXISTS prevent_last_owner_removal();
DROP FUNCTION IF EXISTS reject_movement_on_closed_session();
DROP FUNCTION IF EXISTS reject_closed_cash_session_update();
DROP FUNCTION IF EXISTS reject_sale_mutation();
DROP FUNCTION IF EXISTS touch_updated_at();
DROP FUNCTION IF EXISTS app_setting_uuid(text);
DROP FUNCTION IF EXISTS is_currency(text);
