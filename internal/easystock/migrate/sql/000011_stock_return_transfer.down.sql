DROP POLICY IF EXISTS stock_transfer_events_tenant ON stock_transfer_events;
DROP TABLE IF EXISTS stock_transfer_events;

DROP POLICY IF EXISTS stock_transfer_lines_tenant ON stock_transfer_lines;
DROP TABLE IF EXISTS stock_transfer_lines;

DROP POLICY IF EXISTS stock_transfers_tenant ON stock_transfers;
DROP TABLE IF EXISTS stock_transfers;

DROP POLICY IF EXISTS stock_supplier_return_lines_tenant ON stock_supplier_return_lines;
DROP TABLE IF EXISTS stock_supplier_return_lines;

DROP POLICY IF EXISTS stock_supplier_returns_tenant ON stock_supplier_returns;
DROP TABLE IF EXISTS stock_supplier_returns;
