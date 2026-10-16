DROP INDEX IF EXISTS cash_tx_session_time_idx;
DROP INDEX IF EXISTS cash_sessions_store_opened_idx;

ALTER TABLE cash_sessions DROP COLUMN IF EXISTS close_note;

ALTER TABLE cash_transactions DROP CONSTRAINT IF EXISTS cash_transactions_amount_chk;
ALTER TABLE cash_transactions
  ADD CONSTRAINT cash_transactions_amount_check CHECK (amount > 0);
