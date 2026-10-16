package commerce

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

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
	// ProductUnitID selects the packaging when the name is omitted.
	// The pointer stays nil so an older idempotency hash is unchanged.
	ProductUnitID *uuid.UUID `json:"productUnitId,omitempty"`
}

type ReceiveStockInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	SupplierID     *uuid.UUID
	SupplierName   string
	Currency       string
	Lines          []ReceiveLine
	Reference      string `json:"reference,omitempty"`
	Note           string `json:"note,omitempty"`
}

type ReceiveResultLine struct {
	ProductID     string `json:"productId"`
	ProductUnitID string `json:"productUnitId"`
	UnitName      string `json:"unitName"`
	Quantity      string `json:"quantity"`
	BaseQuantity  string `json:"baseQuantity"`
	UnitCost      string `json:"unitCost"`
	LineCost      string `json:"lineCost"`
}

type ReceiveResult struct {
	PurchaseID string              `json:"purchaseId"`
	StoreID    string              `json:"storeId,omitempty"`
	Currency   string              `json:"currency,omitempty"`
	TotalCost  string              `json:"totalCost"`
	SupplierID string              `json:"supplierId,omitempty"`
	Reference  string              `json:"reference,omitempty"`
	Note       string              `json:"note,omitempty"`
	Lines      []ReceiveResultLine `json:"lines,omitempty"`
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
			if err := json.Unmarshal(replay, &out); err != nil {
				return err
			}
			if out.StoreID == "" || len(out.Lines) == 0 {
				id, err := uuid.Parse(out.PurchaseID)
				if err != nil {
					return err
				}
				out, err = loadReceipt(ctx, tx, id)
				return err
			}
			return nil
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
	reference := strings.TrimSpace(in.Reference)
	note := strings.TrimSpace(in.Note)
	if !validCurrency(in.Currency) || len(in.Lines) == 0 {
		return ReceiveResult{}, domain.Invalid("receive", "Indiquez la devise et au moins un article reçu.")
	}
	if len(reference) > 80 {
		return ReceiveResult{}, domain.Invalid("reference", "La référence est trop longue.")
	}
	if len(note) > 500 {
		return ReceiveResult{}, domain.Invalid("note", "La note est trop longue.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockReceive); err != nil {
		return ReceiveResult{}, err
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return ReceiveResult{}, domain.NotFound("Boutique introuvable.")
	}
	supplierID := in.SupplierID
	if supplierID != nil {
		var supplierOK bool
		if err := tx.QueryRow(ctx, `
			SELECT true FROM suppliers WHERE id = $1 AND store_id = $2 AND archived_at IS NULL
		`, *supplierID, in.StoreID).Scan(&supplierOK); err != nil || !supplierOK {
			return ReceiveResult{}, domain.NotFound("Fournisseur introuvable.")
		}
	} else if strings.TrimSpace(in.SupplierName) != "" {
		id := uuid.New()
		if _, err := tx.Exec(ctx, `
			INSERT INTO suppliers (id, company_id, store_id, name) VALUES ($1,$2,$3,$4)
		`, id, p.CompanyID, in.StoreID, strings.TrimSpace(in.SupplierName)); err != nil {
			return ReceiveResult{}, err
		}
		supplierID = &id
	}
	purchaseID := uuid.New()
	total := decimal.Zero
	type line struct {
		product, unit      uuid.UUID
		name               string
		qty, base, factor  decimal.Decimal
		unitCost, lineCost decimal.Decimal
		density            *decimal.Decimal
	}
	lines := make([]line, 0, len(in.Lines))
	for _, input := range in.Lines {
		if !input.Quantity.IsPositive() || input.UnitCost.IsNegative() {
			return ReceiveResult{}, domain.Invalid("receive", "La quantité reçue doit être positive et le coût ne peut pas être négatif.")
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM products WHERE id = $1`, input.ProductID).Scan(&status); err != nil {
			return ReceiveResult{}, domain.NotFound("Article introuvable.")
		}
		if status != "active" {
			return ReceiveResult{}, domain.FailedPrecondition("product_archived", "Cet article est archivé.")
		}
		var unitID uuid.UUID
		var factor pgtype.Numeric
		var active, allowPurchase, unitFractional bool
		var unitName, dimension string
		var err error
		if input.ProductUnitID != nil && *input.ProductUnitID != uuid.Nil {
			err = tx.QueryRow(ctx, `
				SELECT id, name, factor, active, allow_purchase, allow_fractional, dimension
				FROM product_units WHERE id = $1 AND product_id = $2
			`, *input.ProductUnitID, input.ProductID).Scan(&unitID, &unitName, &factor, &active, &allowPurchase, &unitFractional, &dimension)
		} else {
			err = tx.QueryRow(ctx, `
				SELECT id, name, factor, active, allow_purchase, allow_fractional, dimension
				FROM product_units WHERE product_id = $1 AND name = $2
			`, input.ProductID, strings.TrimSpace(input.UnitName)).Scan(&unitID, &unitName, &factor, &active, &allowPurchase, &unitFractional, &dimension)
		}
		if err != nil {
			return ReceiveResult{}, domain.NotFound("Conditionnement introuvable.")
		}
		if !active {
			return ReceiveResult{}, domain.FailedPrecondition("unit_archived", "Ce conditionnement est archivé.")
		}
		if !allowPurchase {
			return ReceiveResult{}, domain.FailedPrecondition("unit_not_for_purchase", "Ce conditionnement n'est pas autorisé à l'achat.")
		}
		if err := soldQuantityAllowed(unitFractional, input.Quantity); err != nil {
			return ReceiveResult{}, err
		}
		factorDec, err := money.FromNum(factor)
		if err != nil {
			return ReceiveResult{}, err
		}
		conv, err := loadConversion(ctx, tx, input.ProductID)
		if err != nil {
			return ReceiveResult{}, err
		}
		base, density, err := convertQuantity(input.Quantity, factorDec, dimension, conv)
		if err != nil {
			return ReceiveResult{}, err
		}
		unitCost := money.Round(input.UnitCost)
		lineCost := money.Round(input.Quantity.Mul(unitCost))
		total = total.Add(lineCost)
		lines = append(lines, line{product: input.ProductID, unit: unitID, name: unitName, qty: input.Quantity, base: base, factor: factorDec, unitCost: unitCost, lineCost: lineCost, density: density})
	}
	var supplier any
	supplierText := ""
	if supplierID != nil {
		supplier = *supplierID
		supplierText = supplierID.String()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO purchases (id, company_id, store_id, supplier_id, currency, total_cost, created_by, reference, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, purchaseID, p.CompanyID, in.StoreID, supplier, in.Currency, money.Num(total), p.UserID, blankNull(reference), blankNull(note)); err != nil {
		return ReceiveResult{}, err
	}
	resultLines := make([]ReceiveResultLine, 0, len(lines))
	for _, line := range lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO purchase_items (
				id, company_id, purchase_id, product_id, product_unit_id, quantity, base_quantity, unit_cost, line_cost
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, uuid.New(), p.CompanyID, purchaseID, line.product, line.unit, money.Num(line.qty), money.Num(line.base), money.Num(line.unitCost), money.Num(line.lineCost)); err != nil {
			return ReceiveResult{}, err
		}
		resultLines = append(resultLines, ReceiveResultLine{
			ProductID: line.product.String(), ProductUnitID: line.unit.String(), UnitName: line.name,
			Quantity: line.qty.String(), BaseQuantity: line.base.String(),
			UnitCost: line.unitCost.String(), LineCost: line.lineCost.String(),
		})
	}
	posted := append([]line(nil), lines...)
	sort.SliceStable(posted, func(i, j int) bool {
		if posted[i].product == posted[j].product {
			return posted[i].unit.String() < posted[j].unit.String()
		}
		return posted[i].product.String() < posted[j].product.String()
	})
	for _, line := range posted {
		baseUnitCost := line.unitCost.Div(line.base.Div(line.qty))
		if err := applyReceiptCost(ctx, tx, p, in.StoreID, in.Currency, line.product, line.base, baseUnitCost); err != nil {
			return ReceiveResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (
				id, company_id, store_id, product_id, quantity_delta, movement_type, ref_type, ref_id, actor_user_id,
				origin_unit_id, origin_unit_name, origin_quantity, origin_factor, density_kg_per_liter, origin_provenance
			) VALUES ($1,$2,$3,$4,$5,'receive','purchase',$6,$7,$8,$9,$10,$11,$12,'recorded')
		`, uuid.New(), p.CompanyID, in.StoreID, line.product, money.Num(line.base), purchaseID, p.UserID, line.unit, line.name, money.Num(line.qty), money.Num(line.factor), densityArg(line.density)); err != nil {
			return ReceiveResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &in.StoreID, "stock.receive", "purchase", purchaseID, map[string]string{"total": total.String(), "currency": in.Currency}); err != nil {
		return ReceiveResult{}, err
	}
	return ReceiveResult{
		PurchaseID: purchaseID.String(), StoreID: in.StoreID.String(), Currency: in.Currency,
		TotalCost: total.String(), SupplierID: supplierText, Reference: reference, Note: note, Lines: resultLines,
	}, nil
}

func loadReceipt(ctx context.Context, tx pgx.Tx, id uuid.UUID) (ReceiveResult, error) {
	var out ReceiveResult
	var storeID uuid.UUID
	var supplierID *uuid.UUID
	var total string
	var reference, note *string
	err := tx.QueryRow(ctx, `
		SELECT id, store_id, supplier_id, currency, total_cost::text, reference, note
		FROM purchases WHERE id = $1
	`, id).Scan(&id, &storeID, &supplierID, &out.Currency, &total, &reference, &note)
	if err != nil {
		return ReceiveResult{}, domain.NotFound("Réception introuvable.")
	}
	out.PurchaseID = id.String()
	out.StoreID = storeID.String()
	out.TotalCost = plainDecimal(total)
	if supplierID != nil {
		out.SupplierID = supplierID.String()
	}
	if reference != nil {
		out.Reference = *reference
	}
	if note != nil {
		out.Note = *note
	}
	rows, err := tx.Query(ctx, `
		SELECT i.product_id, i.product_unit_id, u.name, i.quantity::text, i.base_quantity::text, i.unit_cost::text, i.line_cost::text
		FROM purchase_items i
		JOIN product_units u ON u.id = i.product_unit_id
		WHERE i.purchase_id = $1
		ORDER BY u.name, i.id
	`, id)
	if err != nil {
		return ReceiveResult{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var line ReceiveResultLine
		var productID, unitID uuid.UUID
		var qty, base, cost, lineCost string
		if err := rows.Scan(&productID, &unitID, &line.UnitName, &qty, &base, &cost, &lineCost); err != nil {
			return ReceiveResult{}, err
		}
		line.ProductID = productID.String()
		line.ProductUnitID = unitID.String()
		line.Quantity = plainDecimal(qty)
		line.BaseQuantity = plainDecimal(base)
		line.UnitCost = plainDecimal(cost)
		line.LineCost = plainDecimal(lineCost)
		out.Lines = append(out.Lines, line)
	}
	return out, rows.Err()
}

func plainDecimal(raw string) string {
	parsed, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return parsed.String()
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
