package readmodel

import (
	"context"
	"sort"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// StockMovement is one audited quantity change, in the product base unit.
type StockMovement struct {
	ID             uuid.UUID
	ProductID      uuid.UUID
	SKU            string
	ProductName    string
	StoreID        uuid.UUID
	Type           string
	Quantity       string
	OriginUnit     string
	OriginQuantity string
	RefType        string
	RefID          uuid.UUID
	ActorID        uuid.UUID
	Reason         string
	CreatedAt      time.Time
}

// MovementQuery pages the journal of one store.
type MovementQuery struct {
	StoreID   uuid.UUID
	ProductID uuid.UUID
	Type      string
	Page      Page
}

// MoneyTotal is one currency bucket. USD and CDF are never added together.
type MoneyTotal struct {
	Currency string
	Amount   string
}

// StockSummary is the stock position of one store or of every visible store.
type StockSummary struct {
	TotalProducts            int
	ProductsInStock          int
	OutOfStockProducts       int
	LowStockProducts         int
	CostVisible              bool
	StockCostValues          []MoneyTotal
	CostMethod               string
	StockRetailValues        []MoneyTotal
	RetailMethod             string
	IncompleteCostProducts   int
	IncompleteRetailProducts int
	UpdatedAt                time.Time
}

func (s *Service) StockMovements(ctx context.Context, p tenant.Principal, q MovementQuery) ([]StockMovement, error) {
	page, err := q.Page.normalize()
	if err != nil {
		return nil, err
	}
	types, err := MovementTypeFilter(q.Type)
	if err != nil {
		return nil, err
	}
	var out []StockMovement
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := authorizeAny(ctx, tx, p, &q.StoreID, access.StockMovementsRead, access.StockRead, access.ReportsRead, access.SalesCreate, access.StockReceive, access.StockAdjust); err != nil {
			return err
		}
		if err := requireStore(ctx, tx, q.StoreID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT m.id, m.product_id, p.sku, p.name, m.store_id, m.movement_type, m.quantity_delta,
			       m.origin_unit_name, m.origin_quantity, m.ref_type, m.ref_id, m.actor_user_id,
			       COALESCE(m.reason, m.note, ''), m.created_at
			FROM stock_movements m
			JOIN products p ON p.id = m.product_id
			WHERE m.store_id = $1
			  AND ($2::uuid = '00000000-0000-0000-0000-000000000000' OR m.product_id = $2)
			  AND ($3::boolean = false OR m.movement_type = ANY($4::text[]))
			  AND ($5::boolean = false OR (m.created_at, m.id) < ($6, $7))
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $8
		`, q.StoreID, q.ProductID, len(types) > 0, types, page.AfterID != uuid.Nil, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item StockMovement
			var delta pgtype.Numeric
			var originQty pgtype.Numeric
			var origin, refType *string
			var refID, actor *uuid.UUID
			if err := rows.Scan(
				&item.ID, &item.ProductID, &item.SKU, &item.ProductName, &item.StoreID, &item.Type, &delta,
				&origin, &originQty, &refType, &refID, &actor, &item.Reason, &item.CreatedAt,
			); err != nil {
				return err
			}
			quantity, err := money.FromNum(delta)
			if err != nil {
				return err
			}
			item.Quantity = quantity.String()
			item.Type = CanonicalMovementType(item.Type)
			if origin != nil {
				item.OriginUnit = *origin
			}
			if originQty.Valid {
				parsed, err := money.FromNum(originQty)
				if err != nil {
					return err
				}
				item.OriginQuantity = parsed.String()
			}
			if refType != nil {
				item.RefType = *refType
			}
			if refID != nil {
				item.RefID = *refID
			}
			if actor != nil {
				item.ActorID = *actor
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) StockSummary(ctx context.Context, p tenant.Principal, storeID uuid.UUID) (StockSummary, error) {
	var out StockSummary
	err := s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var scope *uuid.UUID
		if storeID != uuid.Nil {
			scope = &storeID
		}
		if _, err := authorizeAny(ctx, tx, p, scope, access.StockRead, access.ReportsRead, access.SalesCreate, access.StockReceive, access.StockAdjust); err != nil {
			return err
		}
		stores, err := summaryStores(ctx, tx, p, storeID)
		if err != nil {
			return err
		}
		out.CostMethod = "weighted_average"
		out.RetailMethod = "base_unit_price"
		out.UpdatedAt = time.Now().UTC()
		out.StockRetailValues = []MoneyTotal{}
		visible, err := costAllowed(ctx, tx, p, storeID)
		if err != nil {
			return err
		}
		out.CostVisible = visible
		if visible {
			out.StockCostValues = []MoneyTotal{}
		}
		rows, err := tx.Query(ctx, `
			SELECT p.id, p.alert_threshold, COALESCE(SUM(b.quantity), 0)
			FROM products p
			LEFT JOIN stock_balances b
			  ON b.product_id = p.id
			 AND b.company_id = p.company_id
			 AND b.store_id = ANY($1::uuid[])
			WHERE p.status = 'active' AND p.archived_at IS NULL
			GROUP BY p.id, p.alert_threshold
		`, stores)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			var threshold, qty pgtype.Numeric
			if err := rows.Scan(&id, &threshold, &qty); err != nil {
				return err
			}
			quantity, err := money.FromNum(qty)
			if err != nil {
				return err
			}
			alert, err := money.FromNum(threshold)
			if err != nil {
				return err
			}
			out.TotalProducts++
			switch {
			case quantity.IsPositive() && quantity.LessThanOrEqual(alert):
				out.ProductsInStock++
				out.LowStockProducts++
			case quantity.IsPositive():
				out.ProductsInStock++
			default:
				out.OutOfStockProducts++
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(stores) == 0 {
			return nil
		}
		var updated pgtype.Timestamptz
		if err := tx.QueryRow(ctx, `
			SELECT MAX(updated_at) FROM stock_balances WHERE store_id = ANY($1::uuid[])
		`, stores).Scan(&updated); err != nil {
			return err
		}
		if updated.Valid {
			out.UpdatedAt = updated.Time
		}
		if visible {
			costs, incomplete, err := costValues(ctx, tx, stores)
			if err != nil {
				return err
			}
			out.StockCostValues = costs
			out.IncompleteCostProducts = incomplete
		}
		retail, missing, err := retailValues(ctx, tx, stores)
		if err != nil {
			return err
		}
		out.StockRetailValues = retail
		out.IncompleteRetailProducts = missing
		return nil
	})
	return out, err
}

func summaryStores(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID) ([]uuid.UUID, error) {
	if storeID != uuid.Nil {
		if err := requireStore(ctx, tx, storeID); err != nil {
			return nil, err
		}
		return []uuid.UUID{storeID}, nil
	}
	var role string
	if err := tx.QueryRow(ctx, `SELECT org_role FROM organization_memberships WHERE user_id = $1`, p.UserID).Scan(&role); err != nil {
		return nil, domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	var rows pgx.Rows
	var err error
	if role == "owner" || role == "organization_admin" {
		rows, err = tx.Query(ctx, `SELECT id FROM stores WHERE archived_at IS NULL ORDER BY id`)
	} else {
		rows, err = tx.Query(ctx, `
			SELECT s.id
			FROM stores s
			JOIN store_assignments a ON a.store_id = s.id AND a.user_id = $1
			WHERE s.archived_at IS NULL
			ORDER BY s.id
		`, p.UserID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func costAllowed(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID) (bool, error) {
	var scope *uuid.UUID
	if storeID != uuid.Nil {
		scope = &storeID
	}
	_, err := access.AuthorizeRead(ctx, tx, p.CompanyID, p.UserID, scope, access.StockValuationRead)
	if err == nil {
		return true, nil
	}
	if de, ok := domain.As(err); ok && (de.Code == domain.CodePermissionDenied || de.Code == domain.CodeFailedPrecondition) {
		_, marginErr := access.AuthorizeRead(ctx, tx, p.CompanyID, p.UserID, scope, access.MarginsRead)
		if marginErr == nil {
			return true, nil
		}
		if de, ok := domain.As(marginErr); ok && (de.Code == domain.CodePermissionDenied || de.Code == domain.CodeFailedPrecondition) {
			return false, nil
		}
		return false, marginErr
	}
	return false, err
}

func costValues(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) ([]MoneyTotal, int, error) {
	rows, err := tx.Query(ctx, `
		SELECT b.cost_currency, SUM(b.quantity * b.avg_cost)
		FROM stock_balances b
		JOIN products p ON p.id = b.product_id AND p.status = 'active'
		WHERE b.store_id = ANY($1::uuid[])
		  AND b.quantity > 0
		  AND b.cost_complete
		  AND b.avg_cost IS NOT NULL
		  AND b.cost_currency IS NOT NULL
		GROUP BY b.cost_currency
	`, stores)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	totals := make([]MoneyTotal, 0)
	for rows.Next() {
		var currency string
		var amount pgtype.Numeric
		if err := rows.Scan(&currency, &amount); err != nil {
			return nil, 0, err
		}
		value, err := money.FromNum(amount)
		if err != nil {
			return nil, 0, err
		}
		totals = append(totals, MoneyTotal{Currency: strings.TrimSpace(currency), Amount: money.Round(value).String()})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var incomplete int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT b.product_id)
		FROM stock_balances b
		JOIN products p ON p.id = b.product_id AND p.status = 'active'
		WHERE b.store_id = ANY($1::uuid[])
		  AND b.quantity > 0
		  AND NOT (b.cost_complete AND b.avg_cost IS NOT NULL AND b.cost_currency IS NOT NULL)
	`, stores).Scan(&incomplete); err != nil {
		return nil, 0, err
	}
	sortMoney(totals)
	return totals, incomplete, nil
}

func retailValues(ctx context.Context, tx pgx.Tx, stores []uuid.UUID) ([]MoneyTotal, int, error) {
	rows, err := tx.Query(ctx, `
		SELECT pr.currency, SUM(b.quantity * pr.unit_price)
		FROM stock_balances b
		JOIN products p ON p.id = b.product_id AND p.status = 'active'
		JOIN product_units u ON u.product_id = b.product_id AND u.is_base
		JOIN product_prices pr
		  ON pr.store_id = b.store_id
		 AND pr.product_unit_id = u.id
		 AND pr.archived_at IS NULL
		WHERE b.store_id = ANY($1::uuid[])
		  AND b.quantity > 0
		GROUP BY pr.currency
	`, stores)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	totals := make([]MoneyTotal, 0)
	for rows.Next() {
		var currency string
		var amount pgtype.Numeric
		if err := rows.Scan(&currency, &amount); err != nil {
			return nil, 0, err
		}
		value, err := money.FromNum(amount)
		if err != nil {
			return nil, 0, err
		}
		totals = append(totals, MoneyTotal{Currency: strings.TrimSpace(currency), Amount: money.Round(value).String()})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var missing int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT b.product_id)
		FROM stock_balances b
		JOIN products p ON p.id = b.product_id AND p.status = 'active'
		WHERE b.store_id = ANY($1::uuid[])
		  AND b.quantity > 0
		  AND NOT EXISTS (
		    SELECT 1
		    FROM product_prices pr
		    JOIN product_units u ON u.id = pr.product_unit_id AND u.is_base
		    WHERE pr.product_id = b.product_id
		      AND pr.store_id = b.store_id
		      AND pr.archived_at IS NULL
		  )
	`, stores).Scan(&missing); err != nil {
		return nil, 0, err
	}
	sortMoney(totals)
	return totals, missing, nil
}

func sortMoney(totals []MoneyTotal) {
	sort.Slice(totals, func(i, j int) bool {
		return currencyRank(totals[i].Currency) < currencyRank(totals[j].Currency)
	})
}

func currencyRank(currency string) int {
	switch currency {
	case "USD":
		return 0
	case "CDF":
		return 1
	default:
		return 2
	}
}
