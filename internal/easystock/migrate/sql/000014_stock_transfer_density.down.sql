ALTER TABLE stock_transfer_lines DROP CONSTRAINT IF EXISTS stock_transfer_lines_density_positive;
ALTER TABLE stock_transfer_lines DROP COLUMN IF EXISTS density_kg_per_liter;
