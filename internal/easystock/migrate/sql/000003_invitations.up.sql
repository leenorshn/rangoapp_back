-- Invitations are local until the invitee presents the token.
-- Clerk remains the source of the organization role once a membership webhook arrives.
-- One company not yet linked to a Clerk organization per creator, so a retried
-- signup returns the same company.

ALTER TABLE companies
  ADD COLUMN created_by uuid REFERENCES users (id);

CREATE UNIQUE INDEX companies_one_unlinked
  ON companies (created_by)
  WHERE clerk_org_id IS NULL AND created_by IS NOT NULL;

CREATE INDEX debts_store_created_idx
  ON debts (company_id, store_id, created_at DESC, id);

CREATE INDEX customers_store_created_idx
  ON customers (company_id, store_id, created_at DESC, id);

CREATE TABLE invitations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  email text NOT NULL,
  org_role text NOT NULL,
  store_id uuid,
  status text NOT NULL DEFAULT 'pending',
  token_hash text NOT NULL UNIQUE,
  invited_by uuid NOT NULL,
  accepted_user_id uuid,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  accepted_at timestamptz,
  CONSTRAINT invitations_email_chk CHECK (email = lower(email) AND position('@' IN email) > 1),
  CONSTRAINT invitations_role_chk CHECK (
    org_role IN ('organization_admin', 'store_manager', 'cashier', 'stockkeeper', 'accountant')
  ),
  CONSTRAINT invitations_status_chk CHECK (status IN ('pending', 'accepted', 'revoked')),
  CONSTRAINT invitations_token_chk CHECK (char_length(token_hash) = 64),
  FOREIGN KEY (company_id, invited_by) REFERENCES organization_memberships (company_id, user_id),
  FOREIGN KEY (company_id, store_id) REFERENCES stores (company_id, id),
  FOREIGN KEY (accepted_user_id) REFERENCES users (id)
);

CREATE UNIQUE INDEX invitations_pending_email
  ON invitations (company_id, email)
  WHERE status = 'pending';

CREATE INDEX invitations_company_status_idx
  ON invitations (company_id, status, created_at DESC);

ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations FORCE ROW LEVEL SECURITY;

CREATE POLICY invitations_tenant ON invitations
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

CREATE POLICY invitations_identity ON invitations
  FOR ALL TO easystock_identity
  USING (true)
  WITH CHECK (true);

GRANT SELECT, INSERT, UPDATE ON invitations TO easystock_app, easystock_identity;
