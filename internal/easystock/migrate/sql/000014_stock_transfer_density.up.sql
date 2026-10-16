-- Snapshot of the kg/litre density used when the transfer line was drafted.
-- Dispatch, receipt and cancellation replay this value. They do not read the product again.
-- Older lines stay null: today's density is not proof of a past conversion.
-- base_quantity is left unchanged.

ALTER TABLE stock_transfer_lines
  ADD COLUMN density_kg_per_liter numeric(20,6),
  ADD CONSTRAINT stock_transfer_lines_density_positive CHECK (
    density_kg_per_liter IS NULL OR density_kg_per_liter > 0
  );
