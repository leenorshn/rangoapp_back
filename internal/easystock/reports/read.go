package reports

import (
	"context"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DebtBook is the open receivable book in one currency. It is not limited to the sales period.
type DebtBook struct {
	Currency      string
	OpenAmount    string
	OverdueAmount string
}

// TopProduct ranks sold units after returns. It does not include cost.
type TopProduct struct {
	SKU       string
	Name      string
	Quantity  string
	NetAmount string
	Currency  string
}

// CashCount sums closed drawer counts in the period.
type CashCount struct {
	Currency       string
	ExpectedAmount string
	CountedAmount  string
	Variance       string
}

// StockAlert is a product at or under its threshold.
type StockAlert struct {
	StoreID  uuid.UUID
	SKU      string
	Name     string
	Quantity string
}

func (s *Service) PaymentsByMethod(ctx context.Context, p tenant.Principal, filter Filter) ([]MethodTotal, error) {
	var out []MethodTotal
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		rows, err := tx.Query(ctx, `
			SELECT method, COALESCE(SUM(amount), 0)
			FROM payments
			WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
			GROUP BY method
			ORDER BY method
		`, stores, filter.Currency, filter.From, filter.To)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item MethodTotal
			var amount pgtype.Numeric
			if err := rows.Scan(&item.Method, &amount); err != nil {
				return err
			}
			item.Currency = filter.Currency
			item.Amount, err = numText(amount)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) OpenDebts(ctx context.Context, p tenant.Principal, filter Filter) (DebtBook, error) {
	out := DebtBook{Currency: filter.Currency, OpenAmount: "0", OverdueAmount: "0"}
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		var open, overdue pgtype.Numeric
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(due_amount), 0),
			       COALESCE(SUM(CASE WHEN due_on IS NOT NULL AND due_on < CURRENT_DATE THEN due_amount ELSE 0 END), 0)
			FROM debts
			WHERE store_id = ANY($1) AND currency = $2 AND status IN ('open', 'partial')
		`, stores, filter.Currency).Scan(&open, &overdue); err != nil {
			return err
		}
		var err error
		out.OpenAmount, err = numText(open)
		if err != nil {
			return err
		}
		out.OverdueAmount, err = numText(overdue)
		return err
	})
	return out, err
}

func (s *Service) Expenses(ctx context.Context, p tenant.Principal, filter Filter) (MoneyTotal, error) {
	out := MoneyTotal{Currency: filter.Currency, Amount: "0"}
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		var amount pgtype.Numeric
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM expenses
			WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
		`, stores, filter.Currency, filter.From, filter.To).Scan(&amount); err != nil {
			return err
		}
		text, err := numText(amount)
		if err != nil {
			return err
		}
		out.Amount = text
		return nil
	})
	return out, err
}

func (s *Service) TopProducts(ctx context.Context, p tenant.Principal, filter Filter) ([]TopProduct, error) {
	var out []TopProduct
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		rows, err := tx.Query(ctx, `
			WITH sold AS (
				SELECT si.product_id, SUM(si.base_quantity) AS qty, SUM(si.line_net) AS net
				FROM sale_items si
				JOIN sales s ON s.id = si.sale_id AND s.company_id = si.company_id
				WHERE s.store_id = ANY($1) AND s.currency = $2 AND s.created_at >= $3 AND s.created_at < $4
				GROUP BY si.product_id
			),
			returned AS (
				SELECT si.product_id, SUM(ri.base_quantity) AS qty, SUM(ri.amount) AS net
				FROM refund_items ri
				JOIN refunds r ON r.id = ri.refund_id AND r.company_id = ri.company_id
				JOIN sale_items si ON si.id = ri.sale_item_id AND si.company_id = ri.company_id
				WHERE r.store_id = ANY($1) AND r.currency = $2 AND r.created_at >= $3 AND r.created_at < $4
				GROUP BY si.product_id
			)
			SELECT p.sku, p.name,
			       COALESCE(sold.qty, 0) - COALESCE(returned.qty, 0),
			       COALESCE(sold.net, 0) - COALESCE(returned.net, 0)
			FROM sold
			FULL OUTER JOIN returned ON returned.product_id = sold.product_id
			JOIN products p ON p.id = COALESCE(sold.product_id, returned.product_id)
			ORDER BY COALESCE(sold.net, 0) - COALESCE(returned.net, 0) DESC, p.sku
			LIMIT 10
		`, stores, filter.Currency, filter.From, filter.To)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item TopProduct
			var qty, net pgtype.Numeric
			if err := rows.Scan(&item.SKU, &item.Name, &qty, &net); err != nil {
				return err
			}
			item.Currency = filter.Currency
			item.Quantity, err = numText(qty)
			if err != nil {
				return err
			}
			item.NetAmount, err = numText(net)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) CashCounts(ctx context.Context, p tenant.Principal, filter Filter) ([]CashCount, error) {
	var out []CashCount
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		rows, err := tx.Query(ctx, `
			SELECT c.currency,
			       COALESCE(SUM(c.expected_amount), 0),
			       COALESCE(SUM(c.counted_amount), 0),
			       COALESCE(SUM(c.variance), 0)
			FROM cash_session_counts c
			JOIN cash_sessions s ON s.id = c.session_id AND s.company_id = c.company_id
			WHERE s.store_id = ANY($1)
			  AND s.status = 'closed'
			  AND s.closed_at >= $2 AND s.closed_at < $3
			  AND c.currency = $4
			GROUP BY c.currency
			ORDER BY c.currency
		`, stores, filter.From, filter.To, filter.Currency)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item CashCount
			var expected, counted, variance pgtype.Numeric
			if err := rows.Scan(&item.Currency, &expected, &counted, &variance); err != nil {
				return err
			}
			item.ExpectedAmount, err = numText(expected)
			if err != nil {
				return err
			}
			item.CountedAmount, err = numText(counted)
			if err != nil {
				return err
			}
			item.Variance, err = numText(variance)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) StockAlerts(ctx context.Context, p tenant.Principal, storeIDs []uuid.UUID) ([]StockAlert, error) {
	var out []StockAlert
	err := s.withStores(ctx, p, storeIDs, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		rows, err := tx.Query(ctx, `
			SELECT b.store_id, p.sku, p.name, b.quantity
			FROM stock_balances b
			JOIN products p ON p.id = b.product_id AND p.company_id = b.company_id
			WHERE b.store_id = ANY($1) AND b.quantity <= p.alert_threshold AND p.status = 'active'
			ORDER BY p.sku, b.store_id
		`, stores)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item StockAlert
			var qty pgtype.Numeric
			if err := rows.Scan(&item.StoreID, &item.SKU, &item.Name, &qty); err != nil {
				return err
			}
			item.Quantity, err = numText(qty)
			if err != nil {
				return err
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) withStores(ctx context.Context, p tenant.Principal, storeIDs []uuid.UUID, fn func(context.Context, pgx.Tx, []uuid.UUID) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return err
	}
	filter := Filter{
		StoreIDs: storeIDs,
		Currency: "USD",
		From:     time.Unix(0, 0).UTC(),
		To:       time.Now().UTC().Add(time.Hour),
	}
	stores, err := resolveStores(ctx, tx, p, filter, access.ReportsRead, false)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx, stores); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func numText(value pgtype.Numeric) (string, error) {
	parsed, err := money.FromNum(value)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}
