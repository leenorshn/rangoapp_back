DROP POLICY IF EXISTS entitlement_grants_identity_insert ON entitlement_grants;
DROP POLICY IF EXISTS store_assignments_identity ON store_assignments;
DROP POLICY IF EXISTS stores_identity_read ON stores;
DROP POLICY IF EXISTS plans_identity_write ON subscription_plans;

ALTER TABLE IF EXISTS subscription_payment_requests DROP CONSTRAINT IF EXISTS payment_requests_renewal_fk;
DROP TABLE IF EXISTS rate_limit_buckets, notification_outbox, push_devices, notification_preferences,
  subscription_renewals, subscription_payment_requests, payment_instructions, auth_grants,
  phone_verification_challenges, refresh_tokens, sessions, phone_migration_issues;

ALTER TABLE subscription_snapshots DROP CONSTRAINT IF EXISTS subscription_lifecycle_chk;
DROP TRIGGER IF EXISTS subscription_snapshots_lifecycle ON subscription_snapshots;
DROP FUNCTION IF EXISTS sync_subscription_lifecycle();
ALTER TABLE subscription_snapshots DROP COLUMN IF EXISTS lifecycle;

ALTER TABLE subscription_plans DROP CONSTRAINT IF EXISTS subscription_plans_currency_chk;
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS currency;
ALTER TABLE subscription_plans DROP COLUMN IF EXISTS duration_months;

DROP TRIGGER IF EXISTS companies_renewal_reference ON companies;
DROP FUNCTION IF EXISTS companies_set_renewal_reference();
DROP INDEX IF EXISTS companies_renewal_reference_uq;
ALTER TABLE companies DROP COLUMN IF EXISTS renewal_reference;

ALTER TABLE organization_memberships DROP CONSTRAINT IF EXISTS memberships_status_chk;
ALTER TABLE organization_memberships ADD CONSTRAINT memberships_status_chk CHECK (status IN ('active', 'invited', 'revoked'));
ALTER TABLE organization_memberships DROP CONSTRAINT IF EXISTS memberships_role_chk;
ALTER TABLE organization_memberships ADD CONSTRAINT memberships_role_chk CHECK (
  org_role IN ('owner', 'organization_admin', 'store_manager', 'cashier', 'stockkeeper', 'accountant')
);

DROP TRIGGER IF EXISTS users_sync_name ON users;
DROP FUNCTION IF EXISTS users_sync_name();
DROP INDEX IF EXISTS users_phone_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_chk;
ALTER TABLE users
  DROP COLUMN IF EXISTS is_platform_admin,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS must_change_password,
  DROP COLUMN IF EXISTS password_hash,
  DROP COLUMN IF EXISTS phone_verified_at,
  DROP COLUMN IF EXISTS legacy_phone,
  DROP COLUMN IF EXISTS full_name;
