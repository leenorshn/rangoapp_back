package readmodel

import (
	"context"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service reads commercial data for the caller's company. Amounts stay decimal strings.
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Page is a descending cursor on (created_at, id) or (updated_at, id).
type Page struct {
	After   time.Time
	AfterID uuid.UUID
	Limit   int
}

func (p Page) normalize() (Page, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.After.IsZero() && p.AfterID == uuid.Nil {
		return p, nil
	}
	if p.After.IsZero() || p.AfterID == uuid.Nil {
		return Page{}, domain.Invalid("cursor", "Le curseur doit contenir la date et l'identifiant du dernier élément.")
	}
	return p, nil
}

// SaleLine is a frozen line without cost.
type SaleLine struct {
	ProductID uuid.UUID
	UnitName  string
	Quantity  string
	UnitPrice string
	LineNet   string
}

// Sale is one confirmed sale.
type Sale struct {
	ID         uuid.UUID
	StoreID    uuid.UUID
	CustomerID *uuid.UUID
	Currency   string
	NetAmount  string
	PaidAmount string
	DueAmount  string
	CreatedAt  time.Time
	Lines      []SaleLine
}

// Customer is a store customer card.
type Customer struct {
	ID          uuid.UUID
	StoreID     uuid.UUID
	Name        string
	Phone       string
	Notes       string
	CreditLimit string
	CreatedAt   time.Time
}

// Debt is an open or settled receivable.
type Debt struct {
	ID              uuid.UUID
	StoreID         uuid.UUID
	CustomerID      uuid.UUID
	CustomerName    string
	SaleID          uuid.UUID
	Currency        string
	OriginalAmount  string
	CollectedAmount string
	DueAmount       string
	DueOn           string
	Status          string
	CreatedAt       time.Time
}

// StockItem is the remaining quantity in base units. Cost is not included.
type StockItem struct {
	ProductID      uuid.UUID
	SKU            string
	Name           string
	BaseUnit       string
	Quantity       string
	AlertThreshold string
	BelowAlert     bool
	UpdatedAt      time.Time
}

// Money is one currency amount.
type Money struct {
	Currency string
	Amount   string
}

// CashRegister is a drawer and its open session, when one exists.
type CashRegister struct {
	ID          uuid.UUID
	Name        string
	OpenSession *CashSession
}

// CashSession is the open drawer. Balances is nil when the caller cannot see the amounts.
type CashSession struct {
	ID        uuid.UUID
	OpenedAt  time.Time
	Balances  []Money
	AmountsOK bool
}

// Counts reports how far signup has gone.
type Counts struct {
	Stores     int
	Products   int
	FirstStore uuid.UUID
}

func (s *Service) Counts(ctx context.Context, p tenant.Principal) (Counts, error) {
	var out Counts
	err := s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if err := activeMember(ctx, tx, p.UserID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM stores WHERE archived_at IS NULL`).Scan(&out.Stores); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM products WHERE status = 'active'`).Scan(&out.Products); err != nil {
			return err
		}
		_ = tx.QueryRow(ctx, `
			SELECT id FROM stores WHERE archived_at IS NULL ORDER BY created_at, id LIMIT 1
		`).Scan(&out.FirstStore)
		return nil
	})
	return out, err
}

func (s *Service) Sales(ctx context.Context, p tenant.Principal, storeID uuid.UUID, page Page) ([]Sale, error) {
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	var out []Sale
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		broad, err := authorizeAny(ctx, tx, p, &storeID, access.ReportsRead, access.SalesCreate)
		if err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		var operator *uuid.UUID
		if !broad {
			id := p.UserID
			operator = &id
		}
		rows, err := tx.Query(ctx, `
			SELECT id, store_id, customer_id, currency, net_amount, paid_amount, due_amount, created_at
			FROM sales
			WHERE store_id = $1
			  AND ($2::uuid IS NULL OR operator_user_id = $2)
			  AND ($3::boolean = false OR (created_at, id) < ($4, $5))
			ORDER BY created_at DESC, id DESC
			LIMIT $6
		`, storeID, operator, page.AfterID != uuid.Nil, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		ids := make([]uuid.UUID, 0)
		for rows.Next() {
			var item Sale
			var net, paid, due pgtype.Numeric
			if err := rows.Scan(&item.ID, &item.StoreID, &item.CustomerID, &item.Currency, &net, &paid, &due, &item.CreatedAt); err != nil {
				return err
			}
			item.NetAmount, err = numText(net)
			if err != nil {
				return err
			}
			item.PaidAmount, err = numText(paid)
			if err != nil {
				return err
			}
			item.DueAmount, err = numText(due)
			if err != nil {
				return err
			}
			out = append(out, item)
			ids = append(ids, item.ID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		lines, err := tx.Query(ctx, `
			SELECT si.sale_id, si.product_id, u.name, si.quantity, si.unit_price, si.line_net
			FROM sale_items si
			JOIN product_units u ON u.id = si.product_unit_id AND u.company_id = si.company_id
			WHERE si.sale_id = ANY($1)
			ORDER BY si.sale_id, si.id
		`, ids)
		if err != nil {
			return err
		}
		defer lines.Close()
		bySale := map[uuid.UUID][]SaleLine{}
		for lines.Next() {
			var saleID uuid.UUID
			var line SaleLine
			var qty, price, net pgtype.Numeric
			if err := lines.Scan(&saleID, &line.ProductID, &line.UnitName, &qty, &price, &net); err != nil {
				return err
			}
			line.Quantity, err = numText(qty)
			if err != nil {
				return err
			}
			line.UnitPrice, err = numText(price)
			if err != nil {
				return err
			}
			line.LineNet, err = numText(net)
			if err != nil {
				return err
			}
			bySale[saleID] = append(bySale[saleID], line)
		}
		if err := lines.Err(); err != nil {
			return err
		}
		for i := range out {
			out[i].Lines = bySale[out[i].ID]
		}
		return nil
	})
	return out, err
}

func (s *Service) Customers(ctx context.Context, p tenant.Principal, storeID uuid.UUID, page Page) ([]Customer, error) {
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	var out []Customer
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := authorizeAny(ctx, tx, p, &storeID, access.ReportsRead, access.SalesCreate, access.DebtsCollect); err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT id, store_id, name, phone, notes, credit_limit, created_at
			FROM customers
			WHERE store_id = $1 AND archived_at IS NULL
			  AND ($2::boolean = false OR (created_at, id) < ($3, $4))
			ORDER BY created_at DESC, id DESC
			LIMIT $5
		`, storeID, page.AfterID != uuid.Nil, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Customer
			var phone, notes *string
			var limit pgtype.Numeric
			if err := rows.Scan(&item.ID, &item.StoreID, &item.Name, &phone, &notes, &limit, &item.CreatedAt); err != nil {
				return err
			}
			if phone != nil {
				item.Phone = *phone
			}
			if notes != nil {
				item.Notes = *notes
			}
			if limit.Valid {
				item.CreditLimit, err = numText(limit)
				if err != nil {
					return err
				}
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) Debts(ctx context.Context, p tenant.Principal, storeID uuid.UUID, status string, page Page) ([]Debt, error) {
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	if status != "" && status != "open" && status != "partial" && status != "settled" {
		return nil, domain.Invalid("status", "Le statut de dette doit être open, partial ou settled.")
	}
	var out []Debt
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := authorizeAny(ctx, tx, p, &storeID, access.ReportsRead, access.DebtsCollect); err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT d.id, d.store_id, d.customer_id, c.name, d.sale_id, d.currency,
			       d.original_amount, d.collected_amount, d.due_amount, d.due_on, d.status, d.created_at
			FROM debts d
			JOIN customers c ON c.id = d.customer_id AND c.company_id = d.company_id
			WHERE d.store_id = $1
			  AND ($2::text = '' OR d.status = $2)
			  AND ($3::boolean = false OR (d.created_at, d.id) < ($4, $5))
			ORDER BY d.created_at DESC, d.id DESC
			LIMIT $6
		`, storeID, status, page.AfterID != uuid.Nil, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item Debt
			var original, collected, due pgtype.Numeric
			var dueOn pgtype.Date
			if err := rows.Scan(
				&item.ID, &item.StoreID, &item.CustomerID, &item.CustomerName, &item.SaleID, &item.Currency,
				&original, &collected, &due, &dueOn, &item.Status, &item.CreatedAt,
			); err != nil {
				return err
			}
			item.OriginalAmount, err = numText(original)
			if err != nil {
				return err
			}
			item.CollectedAmount, err = numText(collected)
			if err != nil {
				return err
			}
			item.DueAmount, err = numText(due)
			if err != nil {
				return err
			}
			if dueOn.Valid {
				item.DueOn = dueOn.Time.Format("2006-01-02")
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) Stock(ctx context.Context, p tenant.Principal, storeID uuid.UUID, page Page) ([]StockItem, error) {
	page, err := page.normalize()
	if err != nil {
		return nil, err
	}
	var out []StockItem
	err = s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := authorizeAny(ctx, tx, p, &storeID, access.ReportsRead, access.SalesCreate, access.StockReceive, access.StockAdjust); err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT p.id, p.sku, p.name, p.base_unit, COALESCE(b.quantity, 0), p.alert_threshold, p.updated_at
			FROM products p
			LEFT JOIN stock_balances b
			  ON b.product_id = p.id AND b.company_id = p.company_id AND b.store_id = $1
			WHERE p.status = 'active' AND p.archived_at IS NULL
			  AND ($2::boolean = false OR (p.updated_at, p.id) < ($3, $4))
			ORDER BY p.updated_at DESC, p.id DESC
			LIMIT $5
		`, storeID, page.AfterID != uuid.Nil, page.After, page.AfterID, page.Limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item StockItem
			var qty, alert pgtype.Numeric
			if err := rows.Scan(&item.ProductID, &item.SKU, &item.Name, &item.BaseUnit, &qty, &alert, &item.UpdatedAt); err != nil {
				return err
			}
			quantity, err := money.FromNum(qty)
			if err != nil {
				return err
			}
			threshold, err := money.FromNum(alert)
			if err != nil {
				return err
			}
			item.Quantity = quantity.String()
			item.AlertThreshold = threshold.String()
			item.BelowAlert = quantity.LessThanOrEqual(threshold)
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) Cash(ctx context.Context, p tenant.Principal, storeID uuid.UUID) ([]CashRegister, error) {
	var out []CashRegister
	err := s.with(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		decision, err := authorizeFirst(ctx, tx, p, &storeID, access.ReportsRead, access.CashOpen, access.CashClose)
		if err != nil {
			return err
		}
		if err := requireStore(ctx, tx, storeID); err != nil {
			return err
		}
		seeAll := access.RoleAllows(decision.Role, access.ReportsRead)
		rows, err := tx.Query(ctx, `
			SELECT r.id, r.name, s.id, s.opened_at, s.opened_by
			FROM cash_registers r
			LEFT JOIN cash_sessions s
			  ON s.register_id = r.id AND s.company_id = r.company_id AND s.status = 'open'
			WHERE r.store_id = $1 AND r.archived_at IS NULL
			ORDER BY r.name, r.id
		`, storeID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item CashRegister
			var sessionID *uuid.UUID
			var openedAt *time.Time
			var openedBy *uuid.UUID
			if err := rows.Scan(&item.ID, &item.Name, &sessionID, &openedAt, &openedBy); err != nil {
				return err
			}
			if sessionID != nil && openedAt != nil && openedBy != nil {
				item.OpenSession = &CashSession{ID: *sessionID, OpenedAt: *openedAt, AmountsOK: seeAll || *openedBy == p.UserID}
			}
			out = append(out, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for i := range out {
			session := out[i].OpenSession
			if session == nil || !session.AmountsOK {
				continue
			}
			balances, err := tx.Query(ctx, `
				SELECT currency, COALESCE(SUM(CASE WHEN direction = 'in' THEN amount ELSE -amount END), 0)
				FROM cash_transactions
				WHERE session_id = $1
				GROUP BY currency
				ORDER BY currency
			`, session.ID)
			if err != nil {
				return err
			}
			for balances.Next() {
				var amount Money
				var value pgtype.Numeric
				if err := balances.Scan(&amount.Currency, &value); err != nil {
					balances.Close()
					return err
				}
				amount.Amount, err = numText(value)
				if err != nil {
					balances.Close()
					return err
				}
				session.Balances = append(session.Balances, amount)
			}
			if err := balances.Err(); err != nil {
				balances.Close()
				return err
			}
			balances.Close()
			if session.Balances == nil {
				session.Balances = []Money{}
			}
		}
		return nil
	})
	return out, err
}

func (s *Service) with(ctx context.Context, p tenant.Principal, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := tenant.Set(ctx, tx, p); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func requireStore(ctx context.Context, tx pgx.Tx, storeID uuid.UUID) error {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, storeID).Scan(&ok)
	if err != nil || !ok {
		return domain.NotFound("Boutique introuvable.")
	}
	return nil
}

func activeMember(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM organization_memberships WHERE user_id = $1`, userID).Scan(&status)
	if err != nil || status != "active" {
		return domain.PermissionDenied("membership", "Vous n'êtes pas membre actif de cette entreprise.")
	}
	return nil
}

// authorizeAny returns true when the first permission is reports.read, so the caller may see every operation of the store.
func authorizeAny(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID *uuid.UUID, perms ...string) (bool, error) {
	decision, err := authorizeFirst(ctx, tx, p, storeID, perms...)
	if err != nil {
		return false, err
	}
	return perms[0] == access.ReportsRead && access.RoleAllows(decision.Role, access.ReportsRead), nil
}

func authorizeFirst(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID *uuid.UUID, perms ...string) (access.Decision, error) {
	var last error
	for _, perm := range perms {
		decision, err := access.AuthorizeRead(ctx, tx, p.CompanyID, p.UserID, storeID, perm)
		if err == nil {
			return decision, nil
		}
		last = err
	}
	if last == nil {
		last = domain.PermissionDenied("read", "Cette consultation ne vous est pas autorisée.")
	}
	return access.Decision{}, last
}

func numText(value pgtype.Numeric) (string, error) {
	parsed, err := money.FromNum(value)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}
