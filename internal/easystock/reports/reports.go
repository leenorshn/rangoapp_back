package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Filter struct {
	StoreIDs []uuid.UUID
	Currency string
	From     time.Time
	To       time.Time
}

type MoneyTotal struct {
	Currency string
	Amount   string
}

type MethodTotal struct {
	Method   string
	Currency string
	Amount   string
}

type Margin struct {
	Currency         string
	NetSales         string
	CostOfGoods      string
	GrossMargin      string
	MarginIncomplete bool
}

func (s *Service) NetSales(ctx context.Context, p tenant.Principal, filter Filter) (MoneyTotal, error) {
	var out MoneyTotal
	err := s.with(ctx, p, filter, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		var sales, refunds pgtype.Numeric
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(net_amount), 0)
			FROM sales
			WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
		`, stores, filter.Currency, filter.From, filter.To).Scan(&sales); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM refunds
			WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
		`, stores, filter.Currency, filter.From, filter.To).Scan(&refunds); err != nil {
			return err
		}
		a, _ := money.FromNum(sales)
		b, _ := money.FromNum(refunds)
		out = MoneyTotal{Currency: filter.Currency, Amount: a.Sub(b).String()}
		return nil
	})
	return out, err
}

func (s *Service) GrossMargin(ctx context.Context, p tenant.Principal, filter Filter) (Margin, error) {
	var out Margin
	err := s.with(ctx, p, filter, access.MarginsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		net, err := s.netInTx(ctx, tx, stores, filter)
		if err != nil {
			return err
		}
		var cost pgtype.Numeric
		var incomplete bool
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(CASE WHEN si.cost_complete THEN si.base_quantity * si.unit_cost ELSE 0 END), 0),
			       COALESCE(BOOL_OR(NOT si.cost_complete), false)
			FROM sale_items si
			JOIN sales s ON s.id = si.sale_id AND s.company_id = si.company_id
			WHERE s.store_id = ANY($1) AND s.currency = $2 AND s.created_at >= $3 AND s.created_at < $4
		`, stores, filter.Currency, filter.From, filter.To).Scan(&cost, &incomplete); err != nil {
			return err
		}
		costDec, _ := money.FromNum(cost)
		netDec, _ := decimal.NewFromString(net.Amount)
		out = Margin{
			Currency:         filter.Currency,
			NetSales:         net.Amount,
			CostOfGoods:      costDec.String(),
			GrossMargin:      netDec.Sub(costDec).String(),
			MarginIncomplete: incomplete,
		}
		return nil
	})
	return out, err
}

func (s *Service) ExportNetSales(ctx context.Context, p tenant.Principal, filter Filter) (string, error) {
	total, err := s.NetSales(ctx, p, filter)
	if err != nil {
		return "", err
	}
	if _, err := s.allow(ctx, p, filter, access.ReportsExport, false); err != nil {
		return "", err
	}
	return "currency,net_sales\n" + total.Currency + "," + total.Amount + "\n", nil
}

func (s *Service) ExportMargin(ctx context.Context, p tenant.Principal, filter Filter) (string, error) {
	if _, err := s.allow(ctx, p, filter, access.ReportsExport, true); err != nil {
		return "", err
	}
	margin, err := s.GrossMargin(ctx, p, filter)
	if err != nil {
		return "", err
	}
	incomplete := "false"
	if margin.MarginIncomplete {
		incomplete = "true"
	}
	return "currency,net_sales,cost_of_goods,gross_margin,margin_incomplete\n" +
		strings.Join([]string{margin.Currency, margin.NetSales, margin.CostOfGoods, margin.GrossMargin, incomplete}, ",") + "\n", nil
}

func (s *Service) netInTx(ctx context.Context, tx pgx.Tx, stores []uuid.UUID, filter Filter) (MoneyTotal, error) {
	var sales, refunds pgtype.Numeric
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(net_amount), 0) FROM sales
		WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
	`, stores, filter.Currency, filter.From, filter.To).Scan(&sales); err != nil {
		return MoneyTotal{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM refunds
		WHERE store_id = ANY($1) AND currency = $2 AND created_at >= $3 AND created_at < $4
	`, stores, filter.Currency, filter.From, filter.To).Scan(&refunds); err != nil {
		return MoneyTotal{}, err
	}
	a, _ := money.FromNum(sales)
	b, _ := money.FromNum(refunds)
	return MoneyTotal{Currency: filter.Currency, Amount: a.Sub(b).String()}, nil
}

func (s *Service) allow(ctx context.Context, p tenant.Principal, filter Filter, perm string, margin bool) ([]uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return nil, err
	}
	stores, err := resolveStores(ctx, tx, p, filter, perm, margin)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return stores, nil
}

func (s *Service) with(ctx context.Context, p tenant.Principal, filter Filter, perm string, margin bool, fn func(context.Context, pgx.Tx, []uuid.UUID) error) error {
	if !validCurrency(filter.Currency) {
		return domain.Invalid("currency", "Choisissez une seule devise. Les montants CDF et USD ne sont pas additionnés.")
	}
	if !filter.To.After(filter.From) {
		return domain.Invalid("period", "La période demandée est vide.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return err
	}
	stores, err := resolveStores(ctx, tx, p, filter, perm, margin)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx, stores); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func resolveStores(ctx context.Context, tx pgx.Tx, p tenant.Principal, filter Filter, perm string, margin bool) ([]uuid.UUID, error) {
	var probe *uuid.UUID
	if len(filter.StoreIDs) == 1 {
		probe = &filter.StoreIDs[0]
	}
	decision, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, probe, perm)
	if err != nil {
		return nil, err
	}
	if margin {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, probe, access.MarginsRead); err != nil {
			return nil, err
		}
	}
	allowed := filter.StoreIDs
	if decision.Role != "owner" && decision.Role != "organization_admin" {
		rows, err := tx.Query(ctx, `SELECT store_id FROM store_assignments WHERE user_id = $1`, p.UserID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		assigned := map[uuid.UUID]struct{}{}
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			assigned[id] = struct{}{}
		}
		if len(allowed) == 0 {
			for id := range assigned {
				allowed = append(allowed, id)
			}
		} else {
			filtered := allowed[:0]
			for _, id := range allowed {
				if _, ok := assigned[id]; ok {
					filtered = append(filtered, id)
				}
			}
			allowed = filtered
		}
	} else if len(allowed) == 0 {
		rows, err := tx.Query(ctx, `SELECT id FROM stores WHERE archived_at IS NULL`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			allowed = append(allowed, id)
		}
	}
	if len(allowed) == 0 {
		return nil, domain.PermissionDenied("store", "Aucune boutique autorisée pour ce rapport.")
	}
	rows, err := tx.Query(ctx, `SELECT id FROM stores WHERE id = ANY($1) AND archived_at IS NULL`, allowed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	visible := make([]uuid.UUID, 0, len(allowed))
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		visible = append(visible, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(visible) == 0 {
		return nil, domain.PermissionDenied("store", "Aucune boutique autorisée pour ce rapport.")
	}
	return visible, nil
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (s *Service) LowStock(ctx context.Context, p tenant.Principal, storeID uuid.UUID) ([]string, error) {
	var lines []string
	err := s.with(ctx, p, Filter{StoreIDs: []uuid.UUID{storeID}, Currency: "USD", From: time.Unix(0, 0).UTC(), To: time.Now().UTC().Add(time.Hour)}, access.ReportsRead, false, func(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) error {
		rows, err := tx.Query(ctx, `
			SELECT p.sku, b.quantity::text
			FROM stock_balances b
			JOIN products p ON p.id = b.product_id AND p.company_id = b.company_id
			WHERE b.store_id = ANY($1) AND b.quantity <= p.alert_threshold AND p.status = 'active'
			ORDER BY p.sku
		`, stores)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sku, qty string
			if err := rows.Scan(&sku, &qty); err != nil {
				return err
			}
			lines = append(lines, fmt.Sprintf("%s,%s", sku, qty))
		}
		return rows.Err()
	})
	return lines, err
}
