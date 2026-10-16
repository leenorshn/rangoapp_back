-- A zero opening float is a real balance, not a missing currency.
-- close_note is the optional remark stored with the physical count.
-- The list indexes follow the session history and the movement journal.

ALTER TABLE cash_transactions DROP CONSTRAINT IF EXISTS cash_transactions_amount_check;

ALTER TABLE cash_transactions ADD CONSTRAINT cash_transactions_amount_chk
  CHECK (amount > 0 OR (kind = 'opening' AND amount = 0));

ALTER TABLE cash_sessions ADD COLUMN close_note text;

CREATE INDEX cash_sessions_store_opened_idx
  ON cash_sessions (company_id, store_id, opened_at DESC, id DESC);

CREATE INDEX cash_tx_session_time_idx
  ON cash_transactions (session_id, created_at, id);
