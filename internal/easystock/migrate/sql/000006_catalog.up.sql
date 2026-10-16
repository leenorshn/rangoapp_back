-- Catalog fields the point of sale needs: description, categories, packaging
-- barcodes and an active flag. Prices stay on product_prices.
-- A packaging change is company-wide, so it moves products.updated_at and every
-- boutique sees it. A price change stays on product_prices.updated_at so another
-- boutique is not pulled into a sync it does not need.

CREATE TABLE product_categories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id uuid NOT NULL REFERENCES companies (id),
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (company_id, id),
  CONSTRAINT product_categories_name_chk CHECK (length(btrim(name)) > 0)
);

CREATE UNIQUE INDEX product_categories_name_uq
  ON product_categories (company_id, lower(name));

CREATE TRIGGER product_categories_touch BEFORE UPDATE ON product_categories
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

ALTER TABLE products
  ADD COLUMN description text,
  ADD COLUMN category_id uuid;

ALTER TABLE products
  ADD CONSTRAINT products_category_fk
  FOREIGN KEY (company_id, category_id) REFERENCES product_categories (company_id, id);

ALTER TABLE product_units
  ADD COLUMN barcode text,
  ADD COLUMN active boolean NOT NULL DEFAULT true,
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE product_units
  ADD CONSTRAINT product_units_base_factor_chk CHECK (NOT is_base OR factor = 1),
  ADD CONSTRAINT product_units_base_active_chk CHECK (NOT is_base OR active);

CREATE UNIQUE INDEX product_units_barcode_uq
  ON product_units (company_id, barcode)
  WHERE barcode IS NOT NULL AND active;

CREATE INDEX product_units_product_idx ON product_units (company_id, product_id);

CREATE TRIGGER product_units_touch BEFORE UPDATE ON product_units
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE OR REPLACE FUNCTION catalog_touch_product() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  UPDATE products
  SET updated_at = now()
  WHERE company_id = COALESCE(NEW.company_id, OLD.company_id)
    AND id = COALESCE(NEW.product_id, OLD.product_id);
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER product_units_touch_product
AFTER INSERT OR UPDATE OR DELETE ON product_units
FOR EACH ROW EXECUTE FUNCTION catalog_touch_product();

ALTER TABLE product_prices
  ADD COLUMN archived_at timestamptz;

CREATE INDEX product_prices_sync_idx
  ON product_prices (company_id, store_id, product_id, updated_at);

ALTER TABLE product_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY product_categories_tenant ON product_categories
  FOR ALL TO easystock_app
  USING (company_id = app_setting_uuid('app.company_id'))
  WITH CHECK (company_id = app_setting_uuid('app.company_id'));

GRANT SELECT, INSERT, UPDATE ON product_categories TO easystock_app;
GRANT EXECUTE ON FUNCTION catalog_touch_product() TO easystock_app, easystock_identity, easystock_job;
