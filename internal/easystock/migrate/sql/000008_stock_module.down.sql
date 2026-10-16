DROP POLICY IF EXISTS inventory_count_lines_tenant ON inventory_count_lines;
DROP TABLE IF EXISTS inventory_count_lines;

DROP INDEX IF EXISTS stock_movements_product_idx;
DROP INDEX IF EXISTS inventory_sessions_one_open;

UPDATE stock_movements SET movement_type = 'adjustment'
WHERE movement_type IN ('inventory_adjustment', 'loss');
UPDATE stock_movements SET movement_type = 'receive'
WHERE movement_type IN ('receipt', 'purchase');
UPDATE stock_movements SET movement_type = 'refund'
WHERE movement_type = 'sale_return';
DELETE FROM stock_movements
WHERE movement_type IN ('purchase_return', 'transfer_in', 'transfer_out');

ALTER TABLE stock_movements
  DROP CONSTRAINT stock_movements_type_chk,
  DROP COLUMN note,
  DROP COLUMN origin_quantity,
  DROP COLUMN origin_unit_name,
  DROP COLUMN origin_unit_id;

ALTER TABLE stock_movements
  ADD CONSTRAINT stock_movements_type_chk CHECK (
    movement_type IN ('receive', 'sale', 'refund', 'adjustment', 'opening')
  );

ALTER TABLE inventory_counts
  DROP CONSTRAINT inventory_counts_variance_currency_chk,
  DROP COLUMN variance_currency,
  DROP COLUMN variance_value,
  DROP COLUMN variance_quantity,
  DROP COLUMN theoretical_quantity;

ALTER TABLE inventory_sessions
  DROP CONSTRAINT inventory_sessions_completed_by_fk,
  DROP CONSTRAINT inventory_sessions_note_len,
  DROP COLUMN completed_by,
  DROP COLUMN note;

ALTER TABLE inventory_sessions DROP CONSTRAINT inventory_status_chk;
ALTER TABLE inventory_sessions
  ADD CONSTRAINT inventory_status_chk CHECK (status IN ('draft', 'completed', 'cancelled'));

DELETE FROM purchase_items
WHERE purchase_id IN (SELECT id FROM purchases WHERE supplier_id IS NULL);
DELETE FROM purchases WHERE supplier_id IS NULL;

ALTER TABLE purchases
  DROP CONSTRAINT purchases_note_len,
  DROP CONSTRAINT purchases_reference_len,
  DROP COLUMN note,
  DROP COLUMN reference,
  ALTER COLUMN supplier_id SET NOT NULL;

ALTER TABLE product_units
  DROP COLUMN allow_fractional,
  DROP COLUMN allow_sale,
  DROP COLUMN allow_purchase;
