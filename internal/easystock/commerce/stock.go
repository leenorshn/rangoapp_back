package commerce

import (
	"context"
	"encoding/json"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type ReceiveLine struct {
	ProductID uuid.UUID
	UnitName  string
	Quantity  decimal.Decimal
	UnitCost  decimal.Decimal
}

type ReceiveStockInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	SupplierID     *uuid.UUID
	SupplierName   string
	Currency       string
	Lines          []ReceiveLine
}

type ReceiveResult struct {
	PurchaseID string `json:"purchaseId"`
	TotalCost  string `json:"totalCost"`
}

type PaySupplierInput struct {
	StoreID       uuid.UUID
	SupplierID    uuid.UUID
	PurchaseID    *uuid.UUID
	Currency      string
	Amount        decimal.Decimal
	Method        string
	CashSessionID *uuid.UUID
}

type InventoryCount struct {
	ProductID uuid.UUID
	Counted   decimal.Decimal
	Reason    string
}

type CompleteInventoryInput struct {
	StoreID uuid.UUID
	Counts  []InventoryCount
}

func (s *Service) ReceiveStock(ctx context.Context, p tenant.Principal, in ReceiveStockInput) (ReceiveResult, error) {
	var out ReceiveResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "receive_stock", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			return json.Unmarshal(replay, &out)
		}
		out, err = receiveStock(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.PurchaseID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "receive_stock", in.IdempotencyKey, id, out)
	})
	return out, err
}

func receiveStock(ctx context.Context, tx pgx.Tx, p tenant.Principal, in ReceiveStockInput) (ReceiveResult, error) {
	if !validCurrency(in.Currency) || len(in.Lines) == 0 {
		return ReceiveResult{}, domain.Invalid("receive", "Indiquez la devise et au moins un article reçu.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockReceive); err != nil {
		return ReceiveResult{}, err
	}
	supplierID := in.SupplierID
	if supplierID == nil {
		if in.SupplierName == "" {
			return ReceiveResult{}, domain.Invalid("supplier", "Indiquez le fournisseur.")
		}
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO suppliers (id, company_id, store_id, name) VALUES ($1,$2,$3,$4)
		`, id, p.CompanyID, in.StoreID, in.SupplierName); err != nil {
			return ReceiveResult{}, err
		}
		supplierID = &id
	}
	purchaseID := uuid.New()
	total := decimal.Zero
	type line struct {
		product, unit                 uuid.UUID
		qty, base, unitCost, lineCost decimal.Decimal
	}
	lines := make([]line, 0, len(in.Lines))
	for _, input := range in.Lines {
		if !input.Quantity.IsPositive() || input.UnitCost.IsNegative() {
			return ReceiveResult{}, domain.Invalid("receive", "La quantité reçue doit être positive et le coût ne peut pas être négatif.")
		}
		var status string
		var fractional bool
		if err := tx.QueryRow(ctx, `SELECT status, allow_fractional FROM products WHERE id = $1`, input.ProductID).Scan(&status, &fractional); err != nil {
			return ReceiveResult{}, domain.NotFound("Article introuvable.")
		}
		if status != "active" {
			return ReceiveResult{}, domain.FailedPrecondition("product_archived", "Cet article est archivé.")
		}
		if !fractional && !input.Quantity.Equal(input.Quantity.Truncate(0)) {
			return ReceiveResult{}, domain.Invalid("quantity", "Cet article n'accepte pas les quantités fractionnaires.")
		}
		var unitID uuid.UUID
		var factor pgtype.Numeric
		if err := tx.QueryRow(ctx, `
			SELECT id, factor FROM product_units WHERE product_id = $1 AND name = $2
		`, input.ProductID, input.UnitName).Scan(&unitID, &factor); err != nil {
			return ReceiveResult{}, domain.NotFound("Conditionnement introuvable.")
		}
		factorDec, err := money.FromNum(factor)
		if err != nil {
			return ReceiveResult{}, err
		}
		base := input.Quantity.Mul(factorDec)
		unitCost := money.Round(input.UnitCost)
		lineCost := money.Round(input.Quantity.Mul(unitCost))
		total = total.Add(lineCost)
		lines = append(lines, line{product: input.ProductID, unit: unitID, qty: input.Quantity, base: base, unitCost: unitCost, lineCost: lineCost})
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO purchases (id, company_id, store_id, supplier_id, currency, total_cost, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, purchaseID, p.CompanyID, in.StoreID, *supplierID, in.Currency, money.Num(total), p.UserID); err != nil {
		return ReceiveResult{}, err
	}
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO purchase_items (
				id, company_id, purchase_id, product_id, product_unit_id, quantity, base_quantity, unit_cost, line_cost
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, uuid.New(), p.CompanyID, purchaseID, line.product, line.unit, money.Num(line.qty), money.Num(line.base), money.Num(line.unitCost), money.Num(line.lineCost)); err != nil {
			return ReceiveResult{}, err
		}
		if err := applyReceiptCost(ctx, tx, p, in.StoreID, in.Currency, line.product, line.base, line.unitCost.Div(line.base.Div(line.qty))); err != nil {
			return ReceiveResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (
				id, company_id, store_id, product_id, quantity_delta, movement_type, ref_type, ref_id, actor_user_id
			) VALUES ($1,$2,$3,$4,$5,'receive','purchase',$6,$7)
		`, uuid.New(), p.CompanyID, in.StoreID, line.product, money.Num(line.base), purchaseID, p.UserID); err != nil {
			return ReceiveResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &in.StoreID, "stock.receive", "purchase", purchaseID, map[string]string{"total": total.String(), "currency": in.Currency}); err != nil {
		return ReceiveResult{}, err
	}
	return ReceiveResult{PurchaseID: purchaseID.String(), TotalCost: total.String()}, nil
}

func applyReceiptCost(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID, currency string, productID uuid.UUID, baseQty, baseUnitCost decimal.Decimal) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,0,false)
		ON CONFLICT (company_id, store_id, product_id) DO NOTHING
	`, p.CompanyID, storeID, productID); err != nil {
		return err
	}
	var qty, avg pgtype.Numeric
	var costCurrency *string
	var complete bool
	if err := tx.QueryRow(ctx, `
		SELECT quantity, avg_cost, cost_currency, cost_complete
		FROM stock_balances
		WHERE store_id = $1 AND product_id = $2
		FOR UPDATE
	`, storeID, productID).Scan(&qty, &avg, &costCurrency, &complete); err != nil {
		return err
	}
	have, err := money.FromNum(qty)
	if err != nil {
		return err
	}
	oldAvg, err := money.FromNum(avg)
	if err != nil {
		return err
	}
	newQty := have.Add(baseQty)
	newAvg := money.Round(baseUnitCost)
	newComplete := true
	newCurrency := currency
	switch {
	case !have.IsPositive() || !avg.Valid:
		newAvg = money.Round(baseUnitCost)
	case costCurrency != nil && *costCurrency != currency:
		newAvg = oldAvg
		newComplete = false
		newCurrency = *costCurrency
	case !complete:
		newAvg = money.Round(baseUnitCost)
	default:
		newAvg = money.Round(have.Mul(oldAvg).Add(baseQty.Mul(baseUnitCost)).Div(newQty))
	}
	_, err = tx.Exec(ctx, `
		UPDATE stock_balances
		SET quantity = $4, avg_cost = $5, cost_currency = $6, cost_complete = $7
		WHERE company_id = $1 AND store_id = $2 AND product_id = $3
	`, p.CompanyID, storeID, productID, money.Num(newQty), money.Num(newAvg), newCurrency, newComplete)
	return err
}

func (s *Service) PaySupplier(ctx context.Context, p tenant.Principal, in PaySupplierInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockReceive); err != nil {
			if _, err2 := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.ExpensesCreate); err2 != nil {
				return err
			}
		}
		amount := money.Round(in.Amount)
		if !amount.IsPositive() || (in.Method != "cash" && in.Method != "external") {
			return domain.Invalid("supplier_payment", "Indiquez un montant et un mode de paiement.")
		}
		decision, _ := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockReceive)
		if in.Method == "cash" {
			role := decision.Role
			if role == "" {
				role = "owner"
			}
			if err := lockCashSession(ctx, tx, p, role, in.StoreID, in.CashSessionID, in.Currency); err != nil {
				return err
			}
		}
		id = uuid.New()
		var session any
		if in.Method == "cash" {
			session = *in.CashSessionID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO supplier_payments (
				id, company_id, store_id, supplier_id, purchase_id, currency, amount, method, cash_session_id, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, id, p.CompanyID, in.StoreID, in.SupplierID, in.PurchaseID, in.Currency, money.Num(amount), in.Method, session, p.UserID); err != nil {
			return err
		}
		if in.Method == "cash" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_transactions (
					id, company_id, session_id, currency, direction, kind, amount, ref_type, ref_id, created_by
				) VALUES ($1,$2,$3,$4,'out','supplier',$5,'supplier_payment',$6,$7)
			`, uuid.New(), p.CompanyID, *in.CashSessionID, in.Currency, money.Num(amount), id, p.UserID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, p, &in.StoreID, "supplier.pay", "supplier_payment", id, map[string]string{"amount": amount.String()})
	})
	return id, err
}

func (s *Service) CompleteInventory(ctx context.Context, p tenant.Principal, in CompleteInventoryInput) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockAdjust); err != nil {
			return err
		}
		if len(in.Counts) == 0 {
			return domain.Invalid("inventory", "Indiquez les quantités comptées.")
		}
		id = uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_sessions (id, company_id, store_id, status, created_by, completed_at)
			VALUES ($1,$2,$3,'completed',$4,now())
		`, id, p.CompanyID, in.StoreID, p.UserID); err != nil {
			return err
		}
		for _, count := range in.Counts {
			if count.Counted.IsNegative() {
				return domain.Invalid("quantity", "La quantité comptée ne peut pas être négative.")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
				VALUES ($1,$2,$3,0,false)
				ON CONFLICT (company_id, store_id, product_id) DO NOTHING
			`, p.CompanyID, in.StoreID, count.ProductID); err != nil {
				return err
			}
			var current pgtype.Numeric
			if err := tx.QueryRow(ctx, `
				SELECT quantity FROM stock_balances WHERE store_id = $1 AND product_id = $2 FOR UPDATE
			`, in.StoreID, count.ProductID).Scan(&current); err != nil {
				return domain.NotFound("Article introuvable dans cette boutique.")
			}
			have, err := money.FromNum(current)
			if err != nil {
				return err
			}
			delta := count.Counted.Sub(have)
			if !delta.IsZero() && count.Reason == "" {
				return domain.Invalid("reason", "Indiquez le motif de l'écart d'inventaire.")
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_counts (id, company_id, inventory_id, product_id, counted_quantity, reason)
				VALUES ($1,$2,$3,$4,$5,$6)
			`, uuid.New(), p.CompanyID, id, count.ProductID, money.Num(count.Counted), blankNull(count.Reason)); err != nil {
				return err
			}
			if delta.IsZero() {
				continue
			}
			if _, err := tx.Exec(ctx, `
				UPDATE stock_balances SET quantity = $4 WHERE company_id = $1 AND store_id = $2 AND product_id = $3
			`, p.CompanyID, in.StoreID, count.ProductID, money.Num(count.Counted)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements (
					id, company_id, store_id, product_id, quantity_delta, movement_type, reason, ref_type, ref_id, actor_user_id
				) VALUES ($1,$2,$3,$4,$5,'adjustment',$6,'inventory',$7,$8)
			`, uuid.New(), p.CompanyID, in.StoreID, count.ProductID, money.Num(delta), count.Reason, id, p.UserID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, p, &in.StoreID, "stock.adjust", "inventory", id, map[string]int{"lines": len(in.Counts)})
	})
	return id, err
}
