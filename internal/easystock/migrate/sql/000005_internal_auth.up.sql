-- Internal authentication, direct employees and manual subscription renewal.
-- Existing rows are kept. Clerk identifiers stay as historical columns.
-- Duplicate phones are copied to legacy_phone and phone_migration_issues, then
-- cleared so the unique index can be created. Accounts are not merged.

ALTER TABLE users
  ADD COLUMN full_name text,
  ADD COLUMN legacy_phone text,
  ADD COLUMN phone_verified_at timestamptz,
  ADD COLUMN password_hash text,
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false,
  ADD COLUMN status text NOT NULL DEFAULT 'active',
  ADD COLUMN is_platform_admin boolean NOT NULL DEFAULT false;

UPDATE users SET full_name = display_name WHERE full_name IS NULL;
UPDATE users SET legacy_phone = phone WHERE phone IS NOT NULL AND legacy_phone IS NULL;
UPDATE users
SET must_change_password = true
WHERE clerk_user_id IS NOT NULL AND password_hash IS NULL;

ALTER TABLE users
  ADD CONSTRAINT users_status_chk CHECK (status IN ('active', 'disabled'));

CREATE OR REPLACE FUNCTION users_sync_name() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF NEW.full_name IS NULL OR btrim(NEW.full_name) = '' THEN
      NEW.full_name := NEW.display_name;
    ELSIF NEW.display_name IS NULL OR btrim(NEW.display_name) = '' THEN
      NEW.display_name := NEW.full_name;
    END IF;
  ELSE
    IF NEW.display_name IS DISTINCT FROM OLD.display_name AND NEW.full_name IS NOT DISTINCT FROM OLD.full_name THEN
      NEW.full_name := NEW.display_name;
    ELSIF NEW.full_name IS DISTINCT FROM OLD.full_name AND NEW.display_name IS NOT DISTINCT FROM OLD.display_name THEN
      NEW.display_name := NEW.full_name;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER users_sync_name BEFORE INSERT OR UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION users_sync_name();

ALTER TABLE users ALTER COLUMN full_name SET NOT NULL;

CREATE TABLE phone_migration_issues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id),
  phone text,
  issue text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT phone_migration_issue_chk CHECK (issue IN ('missing', 'duplicate', 'not_e164'))
);

CREATE INDEX phone_migration_issues_open_idx ON phone_migration_issues (issue, user_id);

INSERT INTO phone_migration_issues (user_id, phone, issue)
SELECT id, phone, 'missing'
FROM users
WHERE phone IS NULL OR btrim(phone) = '';

INSERT INTO phone_migration_issues (user_id, phone, issue)
SELECT id, phone, 'duplicate'
FROM users
WHERE phone IS NOT NULL AND btrim(phone) <> '' AND phone IN (
  SELECT phone FROM users WHERE phone IS NOT NULL AND btrim(phone) <> '' GROUP BY phone HAVING count(*) > 1
);

INSERT INTO phone_migration_issues (user_id, phone, issue)
SELECT id, phone, 'not_e164'
FROM users
WHERE phone IS NOT NULL AND btrim(phone) <> '' AND phone !~ '^\+[1-9][0-9]{7,14}$'
  AND phone NOT IN (
    SELECT phone FROM users WHERE phone IS NOT NULL GROUP BY phone HAVING count(*) > 1
  );

UPDATE users SET phone = NULL
WHERE phone IS NOT NULL AND btrim(phone) = '';

UPDATE users SET phone = NULL
WHERE phone IS NOT NULL AND phone IN (
  SELECT phone FROM users WHERE phone IS NOT NULL GROUP BY phone HAVING count(*) > 1
);

CREATE UNIQUE INDEX users_phone_unique ON users (phone) WHERE phone IS NOT NULL;

ALTER TABLE organization_memberships DROP CONSTRAINT memberships_role_chk;
ALTER TABLE organization_memberships ADD CONSTRAINT memberships_role_chk CHECK (
  org_role IN ('owner', 'organization_admin', 'store_manager', 'cashier', 'stockkeeper', 'accountant', 'manager', 'seller')
);

ALTER TABLE organization_memberships DROP CONSTRAINT memberships_status_chk;
ALTER TABLE organization_memberships ADD CONSTRAINT memberships_status_chk CHECK (
  status IN ('active', 'invited', 'revoked', 'disabled')
);

ALTER TABLE companies ADD COLUMN renewal_reference text;

CREATE OR REPLACE FUNCTION companies_set_renewal_reference() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.renewal_reference IS NULL OR btrim(NEW.renewal_reference) = '' THEN
    NEW.renewal_reference := 'ES-' || upper(substr(replace(NEW.id::text, '-', ''), 1, 12));
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER companies_renewal_reference BEFORE INSERT ON companies
FOR EACH ROW EXECUTE FUNCTION companies_set_renewal_reference();

UPDATE companies
SET renewal_reference = 'ES-' || upper(substr(replace(id::text, '-', ''), 1, 12))
WHERE renewal_reference IS NULL;

ALTER TABLE companies ALTER COLUMN renewal_reference SET NOT NULL;
CREATE UNIQUE INDEX companies_renewal_reference_uq ON companies (renewal_reference);

ALTER TABLE subscription_plans
  ADD COLUMN duration_months integer NOT NULL DEFAULT 1 CHECK (duration_months > 0),
  ADD COLUMN currency char(3) NOT NULL DEFAULT 'USD';

ALTER TABLE subscription_plans
  ADD CONSTRAINT subscription_plans_currency_chk CHECK (is_currency(currency));

COMMENT ON COLUMN subscription_plans.duration_months IS
  'Calendar months added on approval via date arithmetic, not a fixed number of days.';

ALTER TABLE subscription_snapshots
  ADD COLUMN lifecycle text NOT NULL DEFAULT 'active';

UPDATE subscription_snapshots SET lifecycle = CASE
  WHEN status = 'canceled' THEN 'closed'
  WHEN status = 'expired' THEN 'expired'
  WHEN status IN ('trialing', 'active', 'grace') THEN 'active'
  ELSE 'inactive'
END;

ALTER TABLE subscription_snapshots
  ADD CONSTRAINT subscription_lifecycle_chk CHECK (lifecycle IN ('inactive', 'active', 'expired', 'closed'));

CREATE OR REPLACE FUNCTION sync_subscription_lifecycle() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'canceled' THEN
    NEW.lifecycle := 'closed';
  ELSIF NEW.status = 'expired' THEN
    NEW.lifecycle := 'expired';
  ELSIF NEW.status IN ('trialing', 'active', 'grace') THEN
    NEW.lifecycle := 'active';
  ELSIF NEW.lifecycle IS NULL OR btrim(NEW.lifecycle) = '' THEN
    NEW.lifecycle := 'inactive';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER subscription_snapshots_lifecycle BEFORE INSERT OR UPDATE OF status ON subscription_snapshots
FOR EACH ROW EXECUTE FUNCTION sync_subscription_lifecycle();

CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id),
  company_id uuid REFERENCES companies (id),
  scope text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_refresh_at timestamptz NOT NULL DEFAULT now(),
  absolute_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  user_agent text,
  ip_hash text,
  CONSTRAINT sessions_scope_chk CHECK (scope IN ('full', 'password_change')),
  CONSTRAINT sessions_absolute_chk CHECK (absolute_expires_at > created_at)
);

CREATE INDEX sessions_user_open_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id uuid NOT NULL REFERENCES sessions (id),
  family_id uuid NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens (session_id);

CREATE TABLE phone_verification_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  purpose text NOT NULL,
  phone text NOT NULL,
  code_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  attempts integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 5,
  sent_at timestamptz NOT NULL,
  consumed_at timestamptz,
  ip_hash text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT phone_challenges_purpose_chk CHECK (purpose IN ('signup', 'password_reset'))
);

CREATE INDEX phone_challenges_open_idx
  ON phone_verification_challenges (phone, purpose, sent_at DESC)
  WHERE consumed_at IS NULL;

CREATE TABLE auth_grants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  purpose text NOT NULL,
  phone text NOT NULL,
  token_hash text NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  user_id uuid REFERENCES users (id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT auth_grants_purpose_chk CHECK (purpose IN ('signup', 'password_reset'))
);

CREATE TABLE payment_instructions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_name text NOT NULL,
  destination_account text NOT NULL,
  beneficiary_name text NOT NULL,
  currency char(3) NOT NULL,
  instructions text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payment_instructions_currency_chk CHECK (is_currency(currency))
);

CREATE UNIQUE INDEX payment_instructions_one_active
  ON payment_instructions (currency)
  WHERE active;

CREATE TABLE subscription_payment_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  plan_code text NOT NULL REFERENCES subscription_plans (code),
  expected_amount numeric(20,4) NOT NULL CHECK (expected_amount > 0),
  declared_amount numeric(20,4) NOT NULL CHECK (declared_amount > 0),
  received_amount numeric(20,4),
  currency char(3) NOT NULL,
  operator_name text NOT NULL,
  destination_account text NOT NULL,
  external_reference text NOT NULL,
  proof_object_key text,
  status text NOT NULL,
  decided_by uuid REFERENCES users (id),
  decided_at timestamptz,
  decision_reason text,
  renewal_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT payment_requests_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT payment_requests_status_chk CHECK (status IN ('pending', 'approved', 'rejected')),
  CONSTRAINT payment_requests_received_chk CHECK (received_amount IS NULL OR received_amount > 0)
);

CREATE UNIQUE INDEX payment_requests_reference_uq
  ON subscription_payment_requests (operator_name, destination_account, external_reference)
  WHERE status <> 'rejected';

CREATE INDEX payment_requests_company_idx
  ON subscription_payment_requests (company_id, created_at DESC);

CREATE TABLE subscription_renewals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  plan_code text NOT NULL REFERENCES subscription_plans (code),
  payment_request_id uuid NOT NULL UNIQUE REFERENCES subscription_payment_requests (id),
  amount numeric(20,4) NOT NULL CHECK (amount > 0),
  currency char(3) NOT NULL,
  starts_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT renewals_currency_chk CHECK (is_currency(currency)),
  CONSTRAINT renewals_period_chk CHECK (expires_at > starts_at)
);

CREATE INDEX subscription_renewals_company_idx ON subscription_renewals (company_id, expires_at DESC);

ALTER TABLE subscription_payment_requests
  ADD CONSTRAINT payment_requests_renewal_fk
  FOREIGN KEY (renewal_id) REFERENCES subscription_renewals (id);

CREATE TABLE notification_preferences (
  company_id uuid PRIMARY KEY REFERENCES companies (id),
  sms_reminders_enabled boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE push_devices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id),
  platform text NOT NULL,
  token text NOT NULL,
  token_hash text NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT push_devices_platform_chk CHECK (platform IN ('android', 'ios', 'web')),
  UNIQUE (user_id, token_hash)
);

CREATE TABLE notification_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  user_id uuid REFERENCES users (id),
  kind text NOT NULL,
  channel text NOT NULL,
  subscription_expires_at timestamptz NOT NULL,
  local_date date NOT NULL,
  slot text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL,
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL,
  provider_id text,
  last_error text,
  idempotency_key text NOT NULL UNIQUE,
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT outbox_channel_chk CHECK (channel IN ('sms', 'push', 'in_app')),
  CONSTRAINT outbox_status_chk CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped', 'unknown', 'cancelled')),
  UNIQUE (company_id, subscription_expires_at, local_date, slot, channel)
);

CREATE INDEX notification_outbox_due_idx
  ON notification_outbox (next_attempt_at)
  WHERE status IN ('pending', 'failed');

CREATE INDEX notification_outbox_user_idx
  ON notification_outbox (user_id, created_at DESC)
  WHERE channel = 'in_app';

CREATE TRIGGER notification_outbox_touch BEFORE UPDATE ON notification_outbox
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TABLE rate_limit_buckets (
  bucket_key text NOT NULL,
  window_start timestamptz NOT NULL,
  hits integer NOT NULL CHECK (hits > 0),
  PRIMARY KEY (bucket_key, window_start)
);

ALTER TABLE phone_migration_issues ENABLE ROW LEVEL SECURITY;
ALTER TABLE phone_migration_issues FORCE ROW LEVEL SECURITY;
CREATE POLICY phone_migration_identity ON phone_migration_issues
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY sessions_identity ON sessions
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY refresh_tokens_identity ON refresh_tokens
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE phone_verification_challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE phone_verification_challenges FORCE ROW LEVEL SECURITY;
CREATE POLICY phone_challenges_identity ON phone_verification_challenges
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE auth_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY auth_grants_identity ON auth_grants
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE payment_instructions ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_instructions FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_instructions_read ON payment_instructions
  FOR SELECT TO easystock_identity, easystock_app, easystock_job USING (true);
CREATE POLICY payment_instructions_identity ON payment_instructions
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

ALTER TABLE subscription_payment_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_payment_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_requests_tenant ON subscription_payment_requests
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));
CREATE POLICY payment_requests_identity ON subscription_payment_requests
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY payment_requests_job ON subscription_payment_requests
  FOR SELECT TO easystock_job USING (true);

ALTER TABLE subscription_renewals ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_renewals FORCE ROW LEVEL SECURITY;
CREATE POLICY renewals_tenant ON subscription_renewals
  FOR SELECT TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'));
CREATE POLICY renewals_identity ON subscription_renewals
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY renewals_job ON subscription_renewals
  FOR SELECT TO easystock_job USING (true);

ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY notification_prefs_tenant ON notification_preferences
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));
CREATE POLICY notification_prefs_identity ON notification_preferences
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY notification_prefs_job ON notification_preferences
  FOR SELECT TO easystock_job USING (true);

ALTER TABLE push_devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE push_devices FORCE ROW LEVEL SECURITY;
CREATE POLICY push_devices_identity ON push_devices
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY push_devices_job ON push_devices
  FOR SELECT TO easystock_job USING (true);

ALTER TABLE notification_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY outbox_identity ON notification_outbox
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY outbox_job ON notification_outbox
  FOR ALL TO easystock_job USING (true) WITH CHECK (true);
CREATE POLICY outbox_app_read ON notification_outbox
  FOR SELECT TO easystock_app
  USING (
    company_id = app_setting_uuid('app.company_id')
    AND (user_id IS NULL OR user_id = app_setting_uuid('app.user_id'))
  );

ALTER TABLE rate_limit_buckets ENABLE ROW LEVEL SECURITY;
ALTER TABLE rate_limit_buckets FORCE ROW LEVEL SECURITY;
CREATE POLICY rate_limit_identity ON rate_limit_buckets
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);
CREATE POLICY rate_limit_job ON rate_limit_buckets
  FOR DELETE TO easystock_job USING (true);

CREATE POLICY users_job_read ON users
  FOR SELECT TO easystock_job USING (true);

CREATE POLICY memberships_job_read ON organization_memberships
  FOR SELECT TO easystock_job USING (true);

CREATE POLICY companies_job_read_all ON companies
  FOR SELECT TO easystock_job USING (true);

CREATE POLICY audit_identity_insert ON audit_logs
  FOR INSERT TO easystock_identity WITH CHECK (true);
CREATE POLICY audit_identity_select ON audit_logs
  FOR SELECT TO easystock_identity USING (true);

GRANT SELECT, INSERT, UPDATE ON users TO easystock_job;
GRANT SELECT ON organization_memberships TO easystock_job;
GRANT SELECT, INSERT, UPDATE ON
  phone_migration_issues, sessions, refresh_tokens, phone_verification_challenges, auth_grants,
  payment_instructions, subscription_payment_requests, subscription_renewals,
  notification_preferences, push_devices, notification_outbox, rate_limit_buckets
TO easystock_identity;

GRANT SELECT ON payment_instructions, subscription_renewals, notification_preferences TO easystock_app;
GRANT SELECT, INSERT ON subscription_payment_requests TO easystock_app;
GRANT SELECT, UPDATE ON notification_outbox TO easystock_app;
GRANT SELECT ON push_devices TO easystock_app;

GRANT SELECT, INSERT, UPDATE ON notification_outbox, notification_preferences TO easystock_job;
GRANT SELECT ON payment_instructions, subscription_payment_requests, subscription_renewals, push_devices TO easystock_job;
GRANT DELETE ON rate_limit_buckets TO easystock_job;
GRANT INSERT ON audit_logs TO easystock_identity;
GRANT SELECT ON audit_logs TO easystock_identity;
GRANT UPDATE ON subscription_plans TO easystock_identity;

CREATE POLICY subscription_snapshots_job_read_all ON subscription_snapshots
  FOR SELECT TO easystock_job
  USING (true);

CREATE POLICY plans_identity_write ON subscription_plans
  FOR UPDATE TO easystock_identity
  USING (true) WITH CHECK (true);
GRANT INSERT, UPDATE ON payment_instructions TO easystock_identity;

GRANT SELECT ON stores TO easystock_identity;
GRANT SELECT, INSERT ON store_assignments TO easystock_identity;
CREATE POLICY stores_identity_read ON stores
  FOR SELECT TO easystock_identity USING (true);
CREATE POLICY store_assignments_identity ON store_assignments
  FOR ALL TO easystock_identity USING (true) WITH CHECK (true);

GRANT INSERT ON entitlement_grants TO easystock_identity;
CREATE POLICY entitlement_grants_identity_insert ON entitlement_grants
  FOR INSERT TO easystock_identity WITH CHECK (true);
