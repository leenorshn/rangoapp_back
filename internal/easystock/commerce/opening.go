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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

type OpeningLine struct {
	ProductID     uuid.UUID
	ProductUnitID uuid.UUID
	Quantity      decimal.Decimal
	UnitCost      *decimal.Decimal `json:"unitCost,omitempty"`
	Currency      string           `json:"currency,omitempty"`
}

type OpenInitialStockInput struct {
	IdempotencyKey string
	StoreID        uuid.UUID
	Note           string
	Lines          []OpeningLine
}

type OpeningResultLine struct {
	ProductID     string `json:"productId"`
	ProductUnitID string `json:"productUnitId"`
	UnitName      string `json:"unitName"`
	Quantity      string `json:"quantity"`
	Factor        string `json:"factor"`
	BaseQuantity  string `json:"baseQuantity"`
	UnitCost      string `json:"unitCost,omitempty"`
	Currency      string `json:"currency,omitempty"`
	LineCost      string `json:"lineCost,omitempty"`
}

type OpeningResult struct {
	OpeningID string              `json:"openingId"`
	StoreID   string              `json:"storeId"`
	Note      string              `json:"note,omitempty"`
	CreatedBy string              `json:"createdBy"`
	CreatedAt time.Time           `json:"createdAt"`
	Lines     []OpeningResultLine `json:"lines"`
}

type stockUnit struct {
	productID  uuid.UUID
	id         uuid.UUID
	name       string
	factor     decimal.Decimal
	dimension  string
	fractional bool
	sku        string
	product    string
	conversion conversion
}

func (u stockUnit) toBase(qty decimal.Decimal) (decimal.Decimal, *decimal.Decimal, error) {
	return convertQuantity(qty, u.factor, u.dimension, u.conversion)
}

type preparedOpening struct {
	unit     stockUnit
	qty      decimal.Decimal
	base     decimal.Decimal
	density  *decimal.Decimal
	cost     *decimal.Decimal
	currency string
	lineCost decimal.Decimal
	hasCost  bool
}

func (s *Service) OpenInitialStock(ctx context.Context, p tenant.Principal, in OpenInitialStockInput) (OpeningResult, error) {
	var out OpeningResult
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		in = normalizeOpening(in)
		hash, err := hashPayload(in)
		if err != nil {
			return err
		}
		replay, err := s.beginIdempotency(ctx, tx, p.CompanyID, "open_initial_stock", in.IdempotencyKey, hash)
		if err != nil {
			return err
		}
		if replay != nil {
			if err := json.Unmarshal(replay, &out); err != nil {
				return err
			}
			if out.OpeningID != "" && len(out.Lines) > 0 {
				return nil
			}
			id, err := uuid.Parse(out.OpeningID)
			if err != nil {
				return err
			}
			out, err = loadOpening(ctx, tx, id)
			return err
		}
		out, err = openInitialStock(ctx, tx, p, in)
		if err != nil {
			return err
		}
		id, err := uuid.Parse(out.OpeningID)
		if err != nil {
			return err
		}
		return finishIdempotency(ctx, tx, p.CompanyID, "open_initial_stock", in.IdempotencyKey, id, out)
	})
	return out, err
}

func normalizeOpening(in OpenInitialStockInput) OpenInitialStockInput {
	in.Note = strings.TrimSpace(in.Note)
	for i := range in.Lines {
		in.Lines[i].Currency = strings.ToUpper(strings.TrimSpace(in.Lines[i].Currency))
		if in.Lines[i].UnitCost != nil {
			rounded := money.Round(*in.Lines[i].UnitCost)
			in.Lines[i].UnitCost = &rounded
		}
	}
	return in
}

func openInitialStock(ctx context.Context, tx pgx.Tx, p tenant.Principal, in OpenInitialStockInput) (OpeningResult, error) {
	if len(in.Note) > 500 {
		return OpeningResult{}, domain.Invalid("note", "La note est trop longue.")
	}
	if len(in.Lines) == 0 {
		return OpeningResult{}, domain.Invalid("opening", "Indiquez au moins un article à ouvrir.")
	}
	if _, err := access.Authorize(ctx, tx, p.CompanyID, p.UserID, &in.StoreID, access.StockOpeningCreate); err != nil {
		return OpeningResult{}, err
	}
	var storeOK bool
	if err := tx.QueryRow(ctx, `SELECT true FROM stores WHERE id = $1 AND archived_at IS NULL`, in.StoreID).Scan(&storeOK); err != nil || !storeOK {
		return OpeningResult{}, domain.NotFound("Boutique introuvable.")
	}
	lines := make([]preparedOpening, 0, len(in.Lines))
	seen := map[string]struct{}{}
	for _, input := range in.Lines {
		if !input.Quantity.IsPositive() {
			return OpeningResult{}, domain.Invalid("quantity", "La quantité initiale doit être strictement positive.")
		}
		if input.ProductUnitID == uuid.Nil || input.ProductID == uuid.Nil {
			return OpeningResult{}, domain.Invalid("opening", "Indiquez l'article et son conditionnement.")
		}
		key := input.ProductID.String() + "/" + input.ProductUnitID.String()
		if _, ok := seen[key]; ok {
			return OpeningResult{}, domain.Invalid("opening", "Ce conditionnement est déjà dans l'ouverture.")
		}
		seen[key] = struct{}{}
		unit, err := loadActiveUnit(ctx, tx, input.ProductID, input.ProductUnitID)
		if err != nil {
			return OpeningResult{}, err
		}
		if err := soldQuantityAllowed(unit.fractional, input.Quantity); err != nil {
			return OpeningResult{}, err
		}
		hasCost := input.UnitCost != nil
		if hasCost && (input.UnitCost.IsNegative() || !validCurrency(input.Currency)) {
			return OpeningResult{}, domain.Invalid("opening", "Le coût initial ne peut pas être négatif et doit indiquer une devise.")
		}
		if !hasCost && input.Currency != "" {
			return OpeningResult{}, domain.Invalid("opening", "Indiquez le coût avec sa devise, ou omettez les deux.")
		}
		base, density, err := unit.toBase(input.Quantity)
		if err != nil {
			return OpeningResult{}, err
		}
		row := preparedOpening{
			unit: unit, qty: input.Quantity, base: base, density: density,
			currency: input.Currency, hasCost: hasCost,
		}
		if hasCost {
			row.cost = input.UnitCost
			row.lineCost = money.Round(input.Quantity.Mul(*input.UnitCost))
		}
		lines = append(lines, row)
	}
	grouped := map[uuid.UUID][]preparedOpening{}
	order := make([]uuid.UUID, 0)
	for _, line := range lines {
		productID := line.unit.productID
		if _, ok := grouped[productID]; !ok {
			order = append(order, productID)
		}
		grouped[productID] = append(grouped[productID], line)
	}
	for _, productID := range order {
		group := grouped[productID]
		var currency string
		var withCost bool
		for i, line := range group {
			if i == 0 {
				withCost = line.hasCost
				currency = line.currency
				continue
			}
			if line.hasCost != withCost || (withCost && line.currency != currency) {
				return OpeningResult{}, domain.Invalid("opening", "Les coûts d'un même article doivent utiliser la même devise.")
			}
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].String() < order[j].String() })
	for _, productID := range order {
		if err := lockOpeningBalance(ctx, tx, p, in.StoreID, productID); err != nil {
			return OpeningResult{}, err
		}
		group := append([]preparedOpening(nil), grouped[productID]...)
		sort.SliceStable(group, func(i, j int) bool { return group[i].unit.id.String() < group[j].unit.id.String() })
		if group[0].hasCost {
			for _, line := range group {
				baseUnitCost := line.cost.Div(line.base.Div(line.qty))
				if err := applyReceiptCost(ctx, tx, p, in.StoreID, line.currency, productID, line.base, baseUnitCost); err != nil {
					return OpeningResult{}, err
				}
			}
			continue
		}
		total := decimal.Zero
		for _, line := range group {
			total = total.Add(line.base)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE stock_balances SET quantity = quantity + $4
			WHERE company_id = $1 AND store_id = $2 AND product_id = $3
		`, p.CompanyID, in.StoreID, productID, money.Num(total)); err != nil {
			return OpeningResult{}, err
		}
	}
	openingID := uuid.New()
	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO stock_openings (id, company_id, store_id, note, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING created_at
	`, openingID, p.CompanyID, in.StoreID, blankNull(in.Note), p.UserID).Scan(&createdAt); err != nil {
		return OpeningResult{}, err
	}
	resultLines := make([]OpeningResultLine, 0, len(lines))
	opened := map[uuid.UUID]struct{}{}
	for _, line := range lines {
		var cost any
		var currency any
		var lineCost any
		result := OpeningResultLine{
			ProductID: line.unit.productID.String(), ProductUnitID: line.unit.id.String(),
			UnitName: line.unit.name, Quantity: line.qty.String(), Factor: line.unit.factor.String(),
			BaseQuantity: line.base.String(),
		}
		if line.hasCost {
			cost = money.Num(*line.cost)
			currency = line.currency
			lineCost = money.Num(line.lineCost)
			result.UnitCost = line.cost.String()
			result.Currency = line.currency
			result.LineCost = line.lineCost.String()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_opening_lines (
				id, company_id, opening_id, product_id, product_unit_id,
				quantity, factor, base_quantity, unit_cost, currency, line_cost
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, uuid.New(), p.CompanyID, openingID, line.unit.productID, line.unit.id,
			money.Num(line.qty), money.Num(line.unit.factor), money.Num(line.base), cost, currency, lineCost); err != nil {
			return OpeningResult{}, err
		}
		if _, ok := opened[line.unit.productID]; !ok {
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_product_openings (company_id, store_id, product_id, opening_id)
				VALUES ($1,$2,$3,$4)
			`, p.CompanyID, in.StoreID, line.unit.productID, openingID); err != nil {
				return OpeningResult{}, err
			}
			opened[line.unit.productID] = struct{}{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (
				id, company_id, store_id, product_id, quantity_delta, movement_type,
				ref_type, ref_id, actor_user_id, origin_unit_id, origin_unit_name, origin_quantity, origin_factor,
				density_kg_per_liter, origin_provenance
			) VALUES ($1,$2,$3,$4,$5,'opening','stock_opening',$6,$7,$8,$9,$10,$11,$12,'recorded')
		`, uuid.New(), p.CompanyID, in.StoreID, line.unit.productID, money.Num(line.base), openingID, p.UserID,
			line.unit.id, line.unit.name, money.Num(line.qty), money.Num(line.unit.factor), densityArg(line.density)); err != nil {
			return OpeningResult{}, err
		}
		resultLines = append(resultLines, result)
	}
	if err := audit(ctx, tx, p, &in.StoreID, "stock.opening.create", "stock_opening", openingID, map[string]int{"lines": len(lines)}); err != nil {
		return OpeningResult{}, err
	}
	return OpeningResult{
		OpeningID: openingID.String(), StoreID: in.StoreID.String(), Note: in.Note,
		CreatedBy: p.UserID.String(), CreatedAt: createdAt, Lines: resultLines,
	}, nil
}

func lockOpeningBalance(ctx context.Context, tx pgx.Tx, p tenant.Principal, storeID, productID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_balances (company_id, store_id, product_id, quantity, cost_complete)
		VALUES ($1,$2,$3,0,false)
		ON CONFLICT (company_id, store_id, product_id) DO NOTHING
	`, p.CompanyID, storeID, productID); err != nil {
		return err
	}
	var qty pgtype.Numeric
	var moved bool
	if err := tx.QueryRow(ctx, `
		SELECT b.quantity, EXISTS (
			SELECT 1 FROM stock_movements m
			WHERE m.store_id = b.store_id AND m.product_id = b.product_id
		)
		FROM stock_balances b
		WHERE b.store_id = $1 AND b.product_id = $2
		FOR UPDATE
	`, storeID, productID).Scan(&qty, &moved); err != nil {
		return err
	}
	have, err := money.FromNum(qty)
	if err != nil {
		return err
	}
	// A zero balance left by an earlier movement is not a fresh start.
	// A positive balance without a movement comes from another path, such as
	// an import, and there is no defined migration exception to replace it.
	if moved || have.IsPositive() {
		return domain.FailedPrecondition("opening_closed", "L'ouverture initiale n'est possible que pour un produit encore jamais mouvementé et sans quantité dans cette boutique.")
	}
	return nil
}

func loadActiveUnit(ctx context.Context, tx pgx.Tx, productID, unitID uuid.UUID) (stockUnit, error) {
	var status, sku, productName string
	if err := tx.QueryRow(ctx, `SELECT status, sku, name FROM products WHERE id = $1`, productID).Scan(&status, &sku, &productName); err != nil {
		return stockUnit{}, domain.NotFound("Article introuvable.")
	}
	if status != "active" {
		return stockUnit{}, domain.FailedPrecondition("product_archived", "Cet article est archivé.")
	}
	var name, dimension string
	var factor pgtype.Numeric
	var active, fractional bool
	err := tx.QueryRow(ctx, `
		SELECT name, factor, active, allow_fractional, dimension
		FROM product_units WHERE id = $1 AND product_id = $2
	`, unitID, productID).Scan(&name, &factor, &active, &fractional, &dimension)
	if err != nil {
		return stockUnit{}, domain.NotFound("Conditionnement introuvable.")
	}
	if !active {
		return stockUnit{}, domain.FailedPrecondition("unit_archived", "Ce conditionnement est archivé.")
	}
	factorDec, err := money.FromNum(factor)
	if err != nil {
		return stockUnit{}, err
	}
	conv, err := loadConversion(ctx, tx, productID)
	if err != nil {
		return stockUnit{}, err
	}
	return stockUnit{
		productID: productID, id: unitID, name: name, factor: factorDec, dimension: dimension,
		fractional: fractional, sku: sku, product: productName, conversion: conv,
	}, nil
}

func loadOpening(ctx context.Context, tx pgx.Tx, id uuid.UUID) (OpeningResult, error) {
	var out OpeningResult
	var storeID, createdBy uuid.UUID
	var note *string
	err := tx.QueryRow(ctx, `
		SELECT id, store_id, note, created_by, created_at FROM stock_openings WHERE id = $1
	`, id).Scan(&id, &storeID, &note, &createdBy, &out.CreatedAt)
	if err != nil {
		return OpeningResult{}, domain.NotFound("Ouverture introuvable.")
	}
	out.OpeningID = id.String()
	out.StoreID = storeID.String()
	out.CreatedBy = createdBy.String()
	if note != nil {
		out.Note = *note
	}
	rows, err := tx.Query(ctx, `
		SELECT product_id, product_unit_id, quantity::text, factor::text, base_quantity::text,
		       unit_cost::text, currency, line_cost::text,
		       (SELECT name FROM product_units u WHERE u.id = product_unit_id)
		FROM stock_opening_lines
		WHERE opening_id = $1
		ORDER BY id
	`, id)
	if err != nil {
		return OpeningResult{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var line OpeningResultLine
		var productID, unitID uuid.UUID
		var qty, factor, base string
		var cost, lineCost *string
		var currency *string
		if err := rows.Scan(&productID, &unitID, &qty, &factor, &base, &cost, &currency, &lineCost, &line.UnitName); err != nil {
			return OpeningResult{}, err
		}
		line.ProductID = productID.String()
		line.ProductUnitID = unitID.String()
		line.Quantity = plainDecimal(qty)
		line.Factor = plainDecimal(factor)
		line.BaseQuantity = plainDecimal(base)
		if cost != nil {
			line.UnitCost = plainDecimal(*cost)
		}
		if currency != nil {
			line.Currency = strings.TrimSpace(*currency)
		}
		if lineCost != nil {
			line.LineCost = plainDecimal(*lineCost)
		}
		out.Lines = append(out.Lines, line)
	}
	return out, rows.Err()
}
