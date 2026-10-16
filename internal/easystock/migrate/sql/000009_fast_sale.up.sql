-- Fast sale: customer address and WhatsApp, phone uniqueness inside one
-- company, and search indexes for the product and customer pickers.
-- debts.due_on already exists. New credit sales must fill it in the application.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE customers
  ADD COLUMN whatsapp_phone text,
  ADD COLUMN address text;

ALTER TABLE customers
  ADD CONSTRAINT customers_address_len CHECK (address IS NULL OR char_length(btrim(address)) BETWEEN 1 AND 300),
  ADD CONSTRAINT customers_whatsapp_len CHECK (whatsapp_phone IS NULL OR char_length(whatsapp_phone) BETWEEN 8 AND 20);

CREATE UNIQUE INDEX customers_company_phone_uq
  ON customers (company_id, phone)
  WHERE phone IS NOT NULL AND archived_at IS NULL;

CREATE UNIQUE INDEX customers_company_whatsapp_uq
  ON customers (company_id, whatsapp_phone)
  WHERE whatsapp_phone IS NOT NULL AND archived_at IS NULL;

CREATE INDEX customers_name_trgm
  ON customers USING gin (name gin_trgm_ops)
  WHERE archived_at IS NULL;

CREATE INDEX customers_phone_trgm
  ON customers USING gin (phone gin_trgm_ops)
  WHERE phone IS NOT NULL AND archived_at IS NULL;

CREATE INDEX products_active_name_trgm
  ON products USING gin (name gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX products_active_sku_trgm
  ON products USING gin (sku gin_trgm_ops)
  WHERE status = 'active';

CREATE INDEX products_active_barcode_trgm
  ON products USING gin (barcode gin_trgm_ops)
  WHERE status = 'active' AND barcode IS NOT NULL;

CREATE INDEX product_units_barcode_trgm
  ON product_units USING gin (barcode gin_trgm_ops)
  WHERE barcode IS NOT NULL AND active;

CREATE INDEX debts_customer_open_idx
  ON debts (company_id, customer_id, currency)
  WHERE due_amount > 0;
