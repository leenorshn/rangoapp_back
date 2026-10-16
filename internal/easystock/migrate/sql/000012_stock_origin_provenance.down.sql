DROP FUNCTION IF EXISTS recover_stock_movement_origins();

ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_origin_provenance_chk;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS origin_provenance;
