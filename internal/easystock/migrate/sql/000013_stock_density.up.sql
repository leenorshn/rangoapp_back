-- Optional kg/litre conversion. Off unless a product enables it.
-- Carton and piece stay quantity × factor. Density is never shared between products.
-- A movement stores the density it used. Later edits do not rewrite that snapshot.

ALTER TABLE products
  ADD COLUMN density_kg_per_liter numeric(20,6),
  ADD COLUMN density_reference text,
  ADD COLUMN density_updated_at timestamptz,
  ADD COLUMN density_updated_by uuid,
  ADD COLUMN cross_dimension_conversion_enabled boolean NOT NULL DEFAULT false;

ALTER TABLE products
  ADD CONSTRAINT products_density_positive CHECK (density_kg_per_liter IS NULL OR density_kg_per_liter > 0),
  ADD CONSTRAINT products_density_reference_len CHECK (
    density_reference IS NULL OR char_length(btrim(density_reference)) BETWEEN 1 AND 200
  ),
  ADD CONSTRAINT products_density_required CHECK (
    NOT cross_dimension_conversion_enabled OR density_kg_per_liter IS NOT NULL
  ),
  ADD CONSTRAINT products_density_updated_by_fkey
    FOREIGN KEY (company_id, density_updated_by) REFERENCES organization_memberships (company_id, user_id);

ALTER TABLE product_units
  ADD COLUMN dimension text NOT NULL DEFAULT 'count',
  ADD CONSTRAINT product_units_dimension_chk CHECK (dimension IN ('count', 'mass', 'volume'));

ALTER TABLE stock_movements
  ADD COLUMN density_kg_per_liter numeric(20,6),
  ADD CONSTRAINT stock_movements_density_positive CHECK (
    density_kg_per_liter IS NULL OR density_kg_per_liter > 0
  );
