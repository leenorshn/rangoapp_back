DROP TABLE IF EXISTS invitations;
DROP INDEX IF EXISTS companies_one_unlinked;
ALTER TABLE companies DROP COLUMN IF EXISTS created_by;
DROP INDEX IF EXISTS debts_store_created_idx;
DROP INDEX IF EXISTS customers_store_created_idx;
