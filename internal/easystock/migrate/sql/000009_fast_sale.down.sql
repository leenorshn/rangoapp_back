DROP INDEX IF EXISTS debts_customer_open_idx;
DROP INDEX IF EXISTS product_units_barcode_trgm;
DROP INDEX IF EXISTS products_active_barcode_trgm;
DROP INDEX IF EXISTS products_active_sku_trgm;
DROP INDEX IF EXISTS products_active_name_trgm;
DROP INDEX IF EXISTS customers_phone_trgm;
DROP INDEX IF EXISTS customers_name_trgm;
DROP INDEX IF EXISTS customers_company_whatsapp_uq;
DROP INDEX IF EXISTS customers_company_phone_uq;

ALTER TABLE customers
  DROP CONSTRAINT IF EXISTS customers_whatsapp_len,
  DROP CONSTRAINT IF EXISTS customers_address_len;

ALTER TABLE customers
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS whatsapp_phone;
