package commerce

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"rangoapp/internal/easystock/access"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/money"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type SupplierReturnLine struct {
	ProductID     uuid.UUID
	ProductUnitID uuid.UUID
	Quantity      decimal.Decimal
}

type SupplierReturnInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	SupplierID     *uuid.UUID `json:"supplierId,omitempty"`
	SupplierName   string     `json:"supplierName,omitempty"`
	Reference      string     `json:"reference,omitempty"`
	Reason         string
	Lines          []SupplierReturnLine
}

type SupplierReturnResultLine struct {
	ProductID     string `json:"productId"`
	ProductUnitID string `json:"productUnitId"`
	UnitName      string `json:"unitName"`
	Quantity      string `json:"quantity"`
	Factor        string `json:"factor"`
	BaseQuantity  string `json:"baseQuantity"`
}

type SupplierReturnResult struct {
	ReturnID   string                     `json:"returnId"`
	StoreID    string                     `json:"storeId"`
	SupplierID string                     `json:"supplierId,omitempty"`
	Reference  string                     `json:"reference,omitempty"`
	Reason     string                     `json:"reason"`
	CreatedAt  time.Time                  `json:"createdAt"`
	Lines      []SupplierReturnResultLine `json:"lines"`
}

func (s *Service) ReturnStockToSupplier(ctx context.Context, p tenant.Principal, in SupplierReturnInput) (SupplierReturnResult, error) {
	var out SupplierReturnResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		in.SupplierName = strings.TrimSpace(in.SupplierName)
		in.Reference = strings.TrimSpace(in.Reference)
		in.Reason = strings.TrimSpace(in.Reason)
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "return_stock_to_supplier", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			if err := json.Unmarshal(replay, &out); err != nil {
				return err
			}
			if out.ReturnID != "" && len(out.Lines) > 0 {
				return nil
			}
			return domain.NotFound("Retour introuvable.")
		}
		out, err = returnStockToSupplier(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.ReturnID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "return_stock_to_supplier", in.IdempotencyKey, id, out)
	})
	return out, err
}

func returnStockToSupplier(ctx context.Context, tx pgx.Tx, p tenant.Principal, in SupplierReturnInput) (SupplierReturnResult, error) {
	if in.Reason == "" || len(in.Reason) > 500 {
		return SupplierReturnResult{}, domain.Invalid("reason", "Indiquez le motif du retour fournisseur.")
	}
	if len(in.Reference) > 80 {
		return SupplierReturnResult{}, domain.Invalid("reference", "La référence est trop longue.")
	}
	if len(in.Lines) == 0 {
		return SupplierReturnResult{}, domain.Invalid("return", "Indiquez au moins un article à retourner.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockReturnCreate); err != nil {
		return SupplierReturnResult{}, err
	}
	if err := requireActiveStore(ctx, tx, in.StoreID); err != nil {
		return SupplierReturnResult{}, err
	}
	supplierID, err := resolveSupplier(ctx, tx, p, in.StoreID, in.SupplierID, in.SupplierName)
	if err != nil {
		return SupplierReturnResult{}, err
	}
	lines, groups, order, err := prepareStockLines(ctx, tx, "return", in.Lines)
	if err != nil {
		return SupplierReturnResult{}, err
	}
	snaps := map[uuid.UUID]balanceSnap{}
	for _, productID := range order {
		snap, err := lockProductBalance(ctx, tx, p.CompanyID, in.StoreID, productID)
		if err != nil {
			return SupplierReturnResult{}, err
		}
		if snap.have.LessThan(groups[productID]) {
			return SupplierReturnResult{}, domain.Conflict("stock_conflict", "Stock insuffisant pour cet article. Réduisez la quantité ou approvisionnez la boutique.")
		}
		snaps[productID] = snap
	}
	returnID := uuid.New()
	var createdAt time.Time
	var supplier any
	supplierText := ""
	if supplierID != nil {
		supplier = *supplierID
		supplierText = supplierID.String()
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO stock_supplier_returns (id, company_id, store_id, supplier_id, reference, reason, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING created_at
	`, returnID, p.CompanyID, in.StoreID, supplier, blankNull(in.Reference), in.Reason, p.UserID).Scan(&createdAt); err != nil {
		return SupplierReturnResult{}, err
	}
	resultLines := make([]SupplierReturnResultLine, 0, len(lines))
	for _, line := range lines {
		snap := snaps[line.unit.productID]
		_, value, currency, ok := snap.valuedExit(line.base)
		var exitValue any
		var exitCurrency any
		var movementValue *decimal.Decimal
		if ok {
			exitValue = money.Num(value)
			exitCurrency = currency
			movementValue = &value
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_supplier_return_lines (
				id, company_id, return_id, product_id, product_unit_id, quantity, factor, base_quantity, exit_value, exit_currency
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, uuid.New(), p.CompanyID, returnID, line.unit.productID, line.unit.id, money.Num(line.qty), money.Num(line.unit.factor), money.Num(line.base), exitValue, exitCurrency); err != nil {
			return SupplierReturnResult{}, err
		}
		if err := postStockMovement(ctx, tx, p, in.StoreID, line.unit.productID, line.base.Neg(), "purchase_return", in.Reason, "stock_supplier_return", returnID, line.unit, line.qty, movementValue, currency); err != nil {
			return SupplierReturnResult{}, err
		}
		resultLines = append(resultLines, SupplierReturnResultLine{
			ProductID: line.unit.productID.String(), ProductUnitID: line.unit.id.String(), UnitName: line.unit.name,
			Quantity: line.qty.String(), Factor: line.unit.factor.String(), BaseQuantity: line.base.String(),
		})
	}
	for _, productID := range order {
		if err := consumeLocked(ctx, tx, p.CompanyID, in.StoreID, productID, groups[productID]); err != nil {
			return SupplierReturnResult{}, err
		}
	}
	if err := audit(ctx, tx, p, &in.StoreID, "stock.return.create", "stock_supplier_return", returnID, map[string]int{"lines": len(lines)}); err != nil {
		return SupplierReturnResult{}, err
	}
	return SupplierReturnResult{
		ReturnID: returnID.String(), StoreID: in.StoreID.String(), SupplierID: supplierText,
		Reference: in.Reference, Reason: in.Reason, CreatedAt: createdAt, Lines: resultLines,
	}, nil
}

type preparedMove struct {
	unit stockUnit
	qty  decimal.Decimal
	base decimal.Decimal
}

func prepareStockLines(ctx context.Context, tx pgx.Tx, kind string, inputs []SupplierReturnLine) ([]preparedMove, map[uuid.UUID]decimal.Decimal, []uuid.UUID, error) {
	if len(inputs) == 0 {
		return nil, nil, nil, domain.Invalid(kind, "Indiquez au moins un article.")
	}
	lines := make([]preparedMove, 0, len(inputs))
	seen := map[string]struct{}{}
	totals := map[uuid.UUID]decimal.Decimal{}
	order := make([]uuid.UUID, 0)
	for _, input := range inputs {
		if !input.Quantity.IsPositive() {
			return nil, nil, nil, domain.Invalid("quantity", "La quantité doit être strictement positive.")
		}
		if input.ProductID == uuid.Nil || input.ProductUnitID == uuid.Nil {
			return nil, nil, nil, domain.Invalid(kind, "Indiquez l'article et son conditionnement.")
		}
		key := input.ProductID.String() + "/" + input.ProductUnitID.String()
		if _, ok := seen[key]; ok {
			return nil, nil, nil, domain.Invalid(kind, "Ce conditionnement est déjà dans l'opération.")
		}
		seen[key] = struct{}{}
		unit, err := loadActiveUnit(ctx, tx, input.ProductID, input.ProductUnitID)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := soldQuantityAllowed(unit.fractional, input.Quantity); err != nil {
			return nil, nil, nil, err
		}
		base := input.Quantity.Mul(unit.factor)
		lines = append(lines, preparedMove{unit: unit, qty: input.Quantity, base: base})
		if _, ok := totals[input.ProductID]; !ok {
			order = append(order, input.ProductID)
		}
		totals[input.ProductID] = totals[input.ProductID].Add(base)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].String() < order[j].String() })
	return lines, totals, order, nil
}

func requireActiveStore(ctx context.Context, tx pgx.Tx, storeID uuid.UUID) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, storeID).Scan(&ok); err != nil || !ok {
		return domain.NotFound("Boutique introuvable.")
	}
	return nil
}

func resolveSupplier(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID uuid.UUID, supplierID *uuid.UUID, name string) (*uuid.UUID, error) {
	if supplierID != nil {
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT true FROM suppliers WHERE id = $1 AND store_id = $2 AND archived_at IS NULL
		`, *supplierID, storeID).Scan(&ok); err != nil || !ok {
			return nil, domain.NotFound("Fournisseur introuvable.")
		}
		return supplierID, nil
	}
	if name == "" {
		return nil, nil
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO suppliers (id, company_id, store_id, name) VALUES ($1,$2,$3,$4)
	`, id, p.CompanyID, storeID, name); err != nil {
		return nil, err
	}
	return &id, nil
}
