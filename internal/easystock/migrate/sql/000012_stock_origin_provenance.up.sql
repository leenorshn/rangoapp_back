-- Origin provenance says whether the packaging was stored with the movement,
-- restored from a linked document, or left unknown.
-- Recovery never changes quantity_delta and never copies today's unit factor.

ALTER TABLE stock_movements
  ADD COLUMN origin_provenance text,
  ADD CONSTRAINT stock_movements_origin_provenance_chk CHECK (
    origin_provenance IS NULL
    OR origin_provenance IN ('recorded', 'recovered', 'legacy_unknown')
  );

CREATE OR REPLACE FUNCTION recover_stock_movement_origins() RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE stock_movements
  SET origin_provenance = 'recorded'
  WHERE origin_provenance IS NULL
    AND (
      origin_unit_id IS NOT NULL
      OR origin_unit_name IS NOT NULL
      OR origin_quantity IS NOT NULL
      OR origin_factor IS NOT NULL
    );

  UPDATE stock_movements m
  SET origin_unit_id = p.unit_id,
      origin_unit_name = p.unit_name,
      origin_quantity = p.origin_qty,
      origin_factor = p.factor,
      origin_provenance = 'recovered'
  FROM (
    SELECT movement_id, unit_id, unit_name, origin_qty, factor
    FROM (
      SELECT m.id AS movement_id,
             i.product_unit_id AS unit_id,
             u.name AS unit_name,
             i.quantity AS origin_qty,
             CASE
               WHEN i.quantity > 0
                AND (i.base_quantity / i.quantity) = round(i.base_quantity / i.quantity, 6)
               THEN round(i.base_quantity / i.quantity, 6)
               ELSE NULL
             END AS factor,
             count(*) OVER (PARTITION BY m.id) AS matches
      FROM stock_movements m
      JOIN purchase_items i
        ON i.company_id = m.company_id
       AND i.purchase_id = m.ref_id
       AND i.product_id = m.product_id
       AND i.base_quantity = abs(m.quantity_delta)
      JOIN product_units u ON u.company_id = m.company_id AND u.id = i.product_unit_id
      WHERE m.origin_provenance IS NULL
        AND m.origin_unit_id IS NULL
        AND m.origin_unit_name IS NULL
        AND m.origin_quantity IS NULL
        AND m.origin_factor IS NULL
        AND m.ref_type = 'purchase'
        AND m.ref_id IS NOT NULL
    ) candidates
    WHERE matches = 1
  ) p
  WHERE m.id = p.movement_id;

  UPDATE stock_movements m
  SET origin_unit_id = p.unit_id,
      origin_unit_name = p.unit_name,
      origin_quantity = p.origin_qty,
      origin_factor = p.factor,
      origin_provenance = 'recovered'
  FROM (
    SELECT movement_id, unit_id, unit_name, origin_qty, factor
    FROM (
      SELECT m.id AS movement_id,
             i.product_unit_id AS unit_id,
             u.name AS unit_name,
             i.quantity AS origin_qty,
             CASE
               WHEN i.quantity > 0
                AND (i.base_quantity / i.quantity) = round(i.base_quantity / i.quantity, 6)
               THEN round(i.base_quantity / i.quantity, 6)
               ELSE NULL
             END AS factor,
             count(*) OVER (PARTITION BY m.id) AS matches
      FROM stock_movements m
      JOIN sale_items i
        ON i.company_id = m.company_id
       AND i.sale_id = m.ref_id
       AND i.product_id = m.product_id
       AND i.base_quantity = abs(m.quantity_delta)
      JOIN product_units u ON u.company_id = m.company_id AND u.id = i.product_unit_id
      WHERE m.origin_provenance IS NULL
        AND m.origin_unit_id IS NULL
        AND m.origin_unit_name IS NULL
        AND m.origin_quantity IS NULL
        AND m.origin_factor IS NULL
        AND m.ref_type = 'sale'
        AND m.ref_id IS NOT NULL
    ) candidates
    WHERE matches = 1
  ) p
  WHERE m.id = p.movement_id;

  UPDATE stock_movements m
  SET origin_unit_id = p.unit_id,
      origin_unit_name = p.unit_name,
      origin_quantity = p.origin_qty,
      origin_factor = p.factor,
      origin_provenance = 'recovered'
  FROM (
    SELECT movement_id, unit_id, unit_name, origin_qty, factor
    FROM (
      SELECT m.id AS movement_id,
             s.product_unit_id AS unit_id,
             u.name AS unit_name,
             r.quantity AS origin_qty,
             CASE
               WHEN r.quantity > 0
                AND (r.base_quantity / r.quantity) = round(r.base_quantity / r.quantity, 6)
               THEN round(r.base_quantity / r.quantity, 6)
               ELSE NULL
             END AS factor,
             count(*) OVER (PARTITION BY m.id) AS matches
      FROM stock_movements m
      JOIN refund_items r
        ON r.company_id = m.company_id
       AND r.refund_id = m.ref_id
       AND r.base_quantity = abs(m.quantity_delta)
      JOIN sale_items s
        ON s.company_id = m.company_id
       AND s.id = r.sale_item_id
       AND s.product_id = m.product_id
      JOIN product_units u ON u.company_id = m.company_id AND u.id = s.product_unit_id
      WHERE m.origin_provenance IS NULL
        AND m.origin_unit_id IS NULL
        AND m.origin_unit_name IS NULL
        AND m.origin_quantity IS NULL
        AND m.origin_factor IS NULL
        AND m.ref_type = 'refund'
        AND m.ref_id IS NOT NULL
    ) candidates
    WHERE matches = 1
  ) p
  WHERE m.id = p.movement_id;

  UPDATE stock_movements
  SET origin_provenance = 'legacy_unknown'
  WHERE origin_provenance IS NULL;
END;
$$;

SELECT recover_stock_movement_origins();
